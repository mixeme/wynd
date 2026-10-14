package archive

import (
	"bufio"
	"bytes"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"filippo.io/age"
	"golang.org/x/crypto/scrypt"
)

// Шифрование полного архива (план 51). Пароль на сервер не уходит: устройство
// выводит из него закрытый ключ X25519 (scrypt — на устройстве чистым JS, без
// WebCrypto, которого нет вне защищённого контекста; к подбору на видеокартах
// он стоек благодаря памяти) и отдаёт серверу только открытый ключ и соль.
// Сервер шифрует на открытый ключ потоком age и расшифровать файл сам не может.
// Соль и параметры лежат в заголовке файла открыто — без них ключ не вывести.
//
// Файл: строка EncryptedMagic, строка JSON EncryptedHeader, затем поток age
// (https://age-encryption.org/v1) с ZIP полного архива внутри.

const (
	EncryptedMagic = "wynd-encrypted/v1"
	// KDFScrypt — вывод ключа из пароля: scrypt с N = 2^LogN, r = 8, p = 1.
	// 2^15 — 32 МБ памяти, на телефоне около секунды; меньше не принимаем.
	KDFScrypt   = "scrypt"
	DefaultLogN = 15
	minLogN     = 15
	maxLogN     = 20
	scryptR     = 8
	scryptP     = 1
	saltLen     = 16
	// ageChunk — кусок потока age: на каждый 16 байт метки.
	ageChunk = 64 << 10
	ageTag   = 16
)

// EncryptedHeader — открытые параметры ключа в начале зашифрованного файла.
type EncryptedHeader struct {
	KDF  string `json:"kdf"`
	LogN int    `json:"log_n"`
	R    int    `json:"r"`
	P    int    `json:"p"`
	Salt string `json:"salt"`
	// Recipient — открытый ключ в записи age («age1…»).
	Recipient string `json:"recipient"`
}

// KeyParams — что устройство прислало для шифрования: открытый ключ X25519,
// соль и стоимость scrypt, с которыми ключ выведен из пароля.
type KeyParams struct {
	PublicKey []byte
	Salt      []byte
	LogN      int
}

// ErrBadKey — открытый ключ или параметры вывода не годятся.
var ErrBadKey = errors.New("archive: bad encryption key")

func (k KeyParams) validate() error {
	if len(k.PublicKey) != 32 || len(k.Salt) != saltLen || k.LogN < minLogN || k.LogN > maxLogN {
		return ErrBadKey
	}
	if _, err := ecdh.X25519().NewPublicKey(k.PublicKey); err != nil {
		return ErrBadKey
	}
	return nil
}

func (k KeyParams) header() (string, error) {
	if err := k.validate(); err != nil {
		return "", err
	}
	rec, err := bech32Encode("age", k.PublicKey)
	if err != nil {
		return "", err
	}
	h, err := json.Marshal(EncryptedHeader{
		KDF: KDFScrypt, LogN: k.LogN, R: scryptR, P: scryptP,
		Salt: base64.StdEncoding.EncodeToString(k.Salt), Recipient: rec,
	})
	if err != nil {
		return "", err
	}
	return EncryptedMagic + "\n" + string(h) + "\n", nil
}

// Encrypt пишет в w заголовок и возвращает поток, в который пишется ZIP.
// Закрытие потока дописывает последний кусок age; w не закрывается.
func Encrypt(w io.Writer, k KeyParams) (io.WriteCloser, error) {
	head, err := k.header()
	if err != nil {
		return nil, err
	}
	rec, err := age.ParseX25519Recipient(mustRecipient(k))
	if err != nil {
		return nil, ErrBadKey
	}
	if _, err := io.WriteString(w, head); err != nil {
		return nil, err
	}
	return age.Encrypt(w, rec)
}

func mustRecipient(k KeyParams) string {
	r, _ := bech32Encode("age", k.PublicKey)
	return r
}

// EncryptedSize — длина зашифрованного файла с открытым содержимым длины
// plain: заголовок файла, заголовок age и по 16 байт метки на кусок.
// Нужна заранее — для Content-Length и места на сервере.
func EncryptedSize(plain int64, k KeyParams) (int64, error) {
	head, err := k.header()
	if err != nil {
		return 0, err
	}
	rec, err := age.ParseX25519Recipient(mustRecipient(k))
	if err != nil {
		return 0, ErrBadKey
	}
	// Заголовок age у одного получателя X25519 — постоянной длины: меряем
	// его на пустом потоке (заголовок, 16 байт nonce, пустой кусок с меткой).
	var cw countWriter
	aw, err := age.Encrypt(&cw, rec)
	if err != nil {
		return 0, err
	}
	if err := aw.Close(); err != nil {
		return 0, err
	}
	fixed := cw.n - ageTag
	chunks := (plain + ageChunk - 1) / ageChunk
	if chunks == 0 {
		chunks = 1
	}
	return int64(len(head)) + fixed + plain + chunks*ageTag, nil
}

// DeriveKey выводит из пароля закрытый ключ X25519 так же, как устройство.
// Сервер её не зовёт — она для проверок и оживления на этой стороне.
func DeriveKey(password string, salt []byte, logN int) (*ecdh.PrivateKey, error) {
	if logN < 1 || logN > maxLogN {
		return nil, ErrBadKey
	}
	raw, err := scrypt.Key([]byte(password), salt, 1<<logN, scryptR, scryptP, 32)
	if err != nil {
		return nil, err
	}
	return ecdh.X25519().NewPrivateKey(raw)
}

// Decrypt читает зашифрованный файл: проверяет заголовок, выводит ключ из
// пароля и отдаёт поток с ZIP внутри.
func Decrypt(r io.Reader, password string) (io.Reader, error) {
	br := bufio.NewReader(r)
	magic, err := br.ReadString('\n')
	if err != nil || strings.TrimSpace(magic) != EncryptedMagic {
		return nil, fmt.Errorf("archive: not an encrypted wynd file")
	}
	line, err := br.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("archive: broken header: %w", err)
	}
	var h EncryptedHeader
	if err := json.Unmarshal(bytes.TrimSpace(line), &h); err != nil || h.KDF != KDFScrypt || h.R != scryptR || h.P != scryptP {
		return nil, fmt.Errorf("archive: broken header")
	}
	salt, err := base64.StdEncoding.DecodeString(h.Salt)
	if err != nil {
		return nil, fmt.Errorf("archive: broken header")
	}
	priv, err := DeriveKey(password, salt, h.LogN)
	if err != nil {
		return nil, err
	}
	secret, err := bech32Encode("age-secret-key-", priv.Bytes())
	if err != nil {
		return nil, err
	}
	id, err := age.ParseX25519Identity(strings.ToUpper(secret))
	if err != nil {
		return nil, err
	}
	return age.Decrypt(br, id)
}

// bech32Encode — запись ключа, которой пользуется age (BIP 173).
func bech32Encode(hrp string, data []byte) (string, error) {
	values, err := convertBits(data, 8, 5)
	if err != nil {
		return "", err
	}
	const charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
	checksum := bech32Checksum(hrp, values)
	var b strings.Builder
	b.WriteString(hrp)
	b.WriteByte('1')
	for _, v := range append(values, checksum...) {
		b.WriteByte(charset[v])
	}
	return b.String(), nil
}

func bech32Polymod(values []byte) uint32 {
	gen := [5]uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	chk := uint32(1)
	for _, v := range values {
		top := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ uint32(v)
		for i := 0; i < 5; i++ {
			if (top>>uint(i))&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}

func bech32Checksum(hrp string, data []byte) []byte {
	var values []byte
	for i := 0; i < len(hrp); i++ {
		values = append(values, hrp[i]>>5)
	}
	values = append(values, 0)
	for i := 0; i < len(hrp); i++ {
		values = append(values, hrp[i]&31)
	}
	values = append(values, data...)
	values = append(values, 0, 0, 0, 0, 0, 0)
	mod := bech32Polymod(values) ^ 1
	out := make([]byte, 6)
	for i := range out {
		out[i] = byte((mod >> uint(5*(5-i))) & 31)
	}
	return out
}

func convertBits(data []byte, from, to uint) ([]byte, error) {
	var acc, bits uint
	var out []byte
	maxv := uint(1)<<to - 1
	for _, b := range data {
		acc = acc<<from | uint(b)
		bits += from
		for bits >= to {
			bits -= to
			out = append(out, byte((acc>>bits)&maxv))
		}
	}
	if bits > 0 {
		out = append(out, byte((acc<<(to-bits))&maxv))
	}
	return out, nil
}
