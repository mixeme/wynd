package archive

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Паритет вёрстки аудио архива с приложением (2026-11-12, C88): стили
// голосового и звука-файла в архиве копируются из web/src/lib/styles/ui.css,
// и тест держит их равными — если значение в приложении меняется, тест падает,
// пока архив не обновят. Раньше равенство держалось только на ручном
// переписывании, и первая версия C88, нарисованная по глазам, дошла до
// владельца с отличиями.
//
// Таблица описывает элементы, а не селекторы: архив верстает те же места
// другими селекторами (в приложении часть значений приходит из общих правил —
// :where(button), .pic, .ic), поэтому слева и справа списки селекторов для
// слияния в порядке каскада. Не сравнивается то, что в архиве осознанно
// другое: margin у .voice/.att (отступы медиа в архиве — на самих строках),
// display у .att-bar (строки собраны на span, в приложении — div),
// cursor у кнопок воспроизведения (в приложении курсор даёт :where(button),
// в архиве он прописан на самих кнопках), color/flex у глобального .ic.
type styleParity struct {
	name  string
	arch  []string
	app   []string
	props []string
}

var audioStyleParity = []styleParity{
	{"строка голосового", []string{".voice"}, []string{".voice"},
		[]string{"display", "align-items", "gap", "width", "padding", "border", "background", "color", "font", "text-align", "cursor"}},
	{"кнопка голосового", []string{".voice-play"}, []string{":where(button)", ".voice-play"},
		[]string{"width", "height", "border-radius", "background", "display", "place-items", "flex", "border", "padding"}},
	{"значок кнопки голосового", []string{".voice-play .ic"}, []string{".voice-play .ic"},
		[]string{"width", "height", "color"}},
	{"волна", []string{".vwave"}, []string{".vwave"},
		[]string{"display", "align-items", "gap", "height", "flex", "min-width", "overflow"}},
	{"полоска волны (ещё не звучало)", []string{".vwave i"}, []string{".vwave i", ".vwave i.off"},
		[]string{"flex", "max-width", "min-height", "border-radius", "background"}},
	{"полоска волны (звучало)", []string{".vwave i.on"}, []string{".vwave i"},
		[]string{"background"}},
	{"строка звука", []string{".att"}, []string{".att"},
		[]string{"display", "align-items", "gap", "border", "border-radius", "padding", "min-width", "box-sizing", "width", "text-align"}},
	{"кнопка воспроизведения", []string{".att-play"}, []string{":where(button)", ".att.audio .att-play"},
		[]string{"display", "align-items", "gap", "flex", "min-width", "text-align"}},
	{"обложка звука", []string{".att-cover"}, []string{".pic", ".att .att-cover"},
		[]string{"width", "height", "border-radius", "flex", "overflow", "display", "border", "background-color", "background-image"}},
	{"обложка звука: файл", []string{".att-cover img"}, []string{".pic img"},
		[]string{"width", "height", "object-fit", "display", "border-radius"}},
	{"тело строки звука", []string{".att-body"}, []string{".att .att-body"},
		[]string{"flex", "min-width", "display", "flex-direction", "justify-content", "gap"}},
	{"строка названия", []string{".att-line"}, []string{".att .att-line"},
		[]string{"display", "align-items", "gap", "min-width"}},
	{"значок в строке звука", []string{".att-line .ic"}, []string{".ic", ".att .att-line .ic"},
		[]string{"width", "height", "flex", "color"}},
	{"название звука", []string{".att-name"}, []string{".att .att-name"},
		[]string{"font-size", "line-height", "flex", "min-width", "overflow", "text-overflow", "white-space"}},
	{"полоса хода", []string{".att-bar"}, []string{".att .att-bar"},
		[]string{"height", "border-radius", "background", "overflow"}},
	{"полоса хода: закраска", []string{".att-bar i"}, []string{".att .att-bar i"},
		[]string{"display", "height", "background", "width"}},
	{"длительность в голосовом", []string{".sz"}, []string{".crow .sz"},
		[]string{"font-size", "color"}},
	{"длительность в звуке", []string{".sz", ".att .sz"}, []string{".att .sz", ".att.audio .sz"},
		[]string{"font-size", "color", "line-height"}},
}

type cssRule struct {
	selectors map[string]bool
	decls     map[string]string
}

var (
	cssComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssSpace   = regexp.MustCompile(`\s+`)
	cssLeadDot = regexp.MustCompile(`([\(,:])\.([0-9])`)
)

// parseCSSRules разбирает таблицу стилей на правила: селекторы (в том числе
// групповые) и объявления. Блоки @media, @font-face, @keyframes пропускаются
// целиком — сравниваются только верхние правила.
func parseCSSRules(css string) []cssRule {
	css = cssComment.ReplaceAllString(css, "")
	var rules []cssRule
	for i := 0; i < len(css); {
		brace := strings.IndexByte(css[i:], '{')
		if brace < 0 {
			break
		}
		sel := cssSpace.ReplaceAllString(css[i:i+brace], " ")
		sel = strings.TrimSpace(sel)
		end, depth := i+brace+1, 1
		for end < len(css) && depth > 0 {
			switch css[end] {
			case '{':
				depth++
			case '}':
				depth--
			}
			end++
		}
		body := css[i+brace+1 : end-1]
		i = end
		if strings.HasPrefix(sel, "@") {
			continue
		}
		decls := map[string]string{}
		for _, d := range strings.Split(body, ";") {
			prop, value, found := strings.Cut(d, ":")
			if !found {
				continue
			}
			prop = strings.TrimSpace(prop)
			value = normalizeCSSValue(value)
			if prop != "" && value != "" {
				decls[prop] = value
			}
		}
		selectors := map[string]bool{}
		for _, s := range strings.Split(sel, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				selectors[s] = true
			}
		}
		rules = append(rules, cssRule{selectors: selectors, decls: decls})
	}
	return rules
}

// normalizeCSSValue сжимает пробелы, дописывает нуль перед дробью (.07 →
// 0.07) и убирает пробелы у запятых и скобок, чтобы сравнивать значения,
// а не формат записи. Пробелы между токенами («0 7px») остаются — они
// значимые.
func normalizeCSSValue(v string) string {
	v = cssSpace.ReplaceAllString(v, " ")
	v = strings.TrimSpace(v)
	v = cssLeadDot.ReplaceAllString(v, "${1}0.$2")
	v = strings.ReplaceAll(v, ", ", ",")
	v = strings.ReplaceAll(v, "( ", "(")
	v = strings.ReplaceAll(v, " )", ")")
	return v
}

// mergeDecls собирает объявления перечисленных селекторов в порядке каскада:
// позднее правило перешибёт раннее, как в браузере.
func mergeDecls(rules []cssRule, selectors []string) map[string]string {
	want := map[string]bool{}
	for _, s := range selectors {
		want[s] = true
	}
	out := map[string]string{}
	for _, r := range rules {
		if !r.matches(want) {
			continue
		}
		for k, v := range r.decls {
			out[k] = v
		}
	}
	return out
}

func (r cssRule) matches(want map[string]bool) bool {
	for s := range r.selectors {
		if want[s] {
			return true
		}
	}
	return false
}

// TestArchiveAudioStylesMatchApp сравнивает стили аудиострок архива с
// web/src/lib/styles/ui.css по списку свойств каждой записи таблицы.
func TestArchiveAudioStylesMatchApp(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "lib", "styles", "ui.css"))
	if err != nil {
		t.Fatalf("нет стилей приложения: %v", err)
	}
	app := parseCSSRules(string(raw))
	arch := parseCSSRules(baseCSS)
	for _, e := range audioStyleParity {
		archDecls := mergeDecls(arch, e.arch)
		appDecls := mergeDecls(app, e.app)
		for _, prop := range e.props {
			got, ok := archDecls[prop]
			if !ok {
				t.Errorf("%s: в архиве нет свойства %s (селекторы %v)", e.name, prop, e.arch)
				continue
			}
			want, ok := appDecls[prop]
			if !ok {
				t.Errorf("%s: в ui.css пропало свойство %s (селекторы %v) — архив разошёлся с приложением", e.name, prop, e.app)
				continue
			}
			if got != want {
				t.Errorf("%s: %s — в архиве %q, в приложении %q", e.name, prop, got, want)
			}
		}
	}
}
