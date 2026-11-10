// Package relver сравнивает номера выпусков программ Wynd (X.Y.Z) и
// выбирает их из списка тегов. Один и тот же разбор нужен Мастерской,
// самообновлению окна и Подписи, поэтому он живёт отдельно от программ.
package relver

import "strings"

// Number разбирает номер вида «0.4.1». Части — до шести цифр, ровно три.
func Number(tag string) ([3]int, bool) {
	var num [3]int
	parts := strings.Split(tag, ".")
	if len(parts) != 3 {
		return num, false
	}
	for i, p := range parts {
		if p == "" || len(p) > 6 {
			return num, false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return num, false
			}
			num[i] = num[i]*10 + int(c-'0')
		}
	}
	return num, true
}

// Less — a старше b.
func Less(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
