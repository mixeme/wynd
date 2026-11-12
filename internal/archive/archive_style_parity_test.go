package archive

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Паритет вёрстки архива с приложением (2026-11-12, C88 и дальше): стили
// архива копируются из web/src/lib/styles/tokens.css и ui.css, и тест держит
// их равными — если значение в приложении меняется, тест падает, пока архив
// не обновят. Раньше равенство держалось только на ручном переписывании, и
// первая версия аудиострок, нарисованная по глазам, дошла до владельца с
// отличиями.
//
// Таблица описывает элементы, а не селекторы: архив верстает те же места
// другими селекторами (в приложении часть значений приходит из общих правил —
// :where(button), .pic, .ic), поэтому слева и справа списки селекторов для
// слияния в порядке каскада. Равенство требуется по перечисленным свойствам;
// расхождение допустимо, только если оно занесено в known — задокументированное
// отличие (свойство → значение архива, "" — свойства в архиве нет). Если
// отличие исчезло (архив выровняли с приложением), тест подскажет убрать
// запись. Новое расхождение, которого нет в таблице, падает сразу.
//
// Элементы без аналога в приложении не сравниваются: .head, .wrap (640 px),
// .toc, .back, .end — навигация и каркас архива; .gone («Файл недоступен»);
// нативный плеер в <noscript> и состояния playing.
//
// Структурные отличия свойствами не выражаются и решаются владельцем по
// плану 46 (A26): приложение ставит фото и ролики плитками-квадратами
// (.thumbs/.pic.sq), архив — столбиком на всю ширину; отклики в приложении —
// чипсы на фоне --ct (.rx), в архиве — строка текста; комментарии в
// приложении — отдельная карточка .cm на бумажном фоне со строками .cmt,
// в архиве — список внутри карточки записи; файл в приложении — карточка
// .att, в архиве — ссылка .cfile. font-family не сравнивается: в приложении
// это var(--ui), текстура стеков различается записью, а не составом.
type styleParity struct {
	name  string
	arch  []string
	app   []string
	props []string
	// known — задокументированные отличия: свойство → значение архива
	// ("" — свойства в архиве нет). Значение справа от свойства в
	// приложении живёт в документе плана, не здесь.
	known map[string]string
	note  string
}

var archiveStyleParity = []styleParity{
	// ——— Равные значения ———
	{"цвета страницы", []string{":root"}, []string{":root"}, []string{"--paper", "--card", "--ink", "--muted", "--faint", "--line"}, nil, "токены tokens.css"},
	{"шапка записи", []string{".who"}, []string{".pa"}, []string{"display", "align-items", "gap"},
		map[string]string{"margin": ""}, "отступ: в приложении 9 px под шапкой, в архиве 10 px над текстом"},
	{"имя автора", []string{".author"}, []string{".pa .n"}, []string{"font-weight"}, nil, ""},
	{"строка голосового", []string{".voice"}, []string{".voice"}, []string{"display", "align-items", "gap", "width", "padding", "border", "background", "color", "font", "text-align", "cursor"}, nil, ""},
	{"кнопка голосового", []string{".voice-play"}, []string{":where(button)", ".voice-play"}, []string{"width", "height", "border-radius", "background", "display", "place-items", "flex", "border", "padding"}, nil, ""},
	{"значок кнопки голосового", []string{".voice-play .ic"}, []string{".voice-play .ic"}, []string{"width", "height", "color"}, nil, ""},
	{"волна", []string{".vwave"}, []string{".vwave"}, []string{"display", "align-items", "gap", "height", "flex", "min-width", "overflow"}, nil, ""},
	{"полоска волны (ещё не звучало)", []string{".vwave i"}, []string{".vwave i", ".vwave i.off"}, []string{"flex", "max-width", "min-height", "border-radius", "background"}, nil, ""},
	{"полоска волны (звучало)", []string{".vwave i.on"}, []string{".vwave i"}, []string{"background"}, nil, ""},
	{"строка звука", []string{".att"}, []string{".att"}, []string{"display", "align-items", "gap", "border", "border-radius", "padding", "min-width", "box-sizing", "width", "text-align"}, nil, ""},
	{"кнопка воспроизведения", []string{".att-play"}, []string{":where(button)", ".att.audio .att-play"}, []string{"display", "align-items", "gap", "flex", "min-width", "text-align"}, nil, ""},
	{"обложка звука", []string{".att-cover"}, []string{".pic", ".att .att-cover"}, []string{"width", "height", "border-radius", "flex", "overflow", "display", "border", "background-color", "background-image"}, nil, ""},
	{"обложка звука: файл", []string{".att-cover img"}, []string{".pic img"}, []string{"width", "height", "object-fit", "display", "border-radius"}, nil, ""},
	{"тело строки звука", []string{".att-body"}, []string{".att .att-body"}, []string{"flex", "min-width", "display", "flex-direction", "justify-content", "gap"}, nil, ""},
	{"строка названия", []string{".att-line"}, []string{".att .att-line"}, []string{"display", "align-items", "gap", "min-width"}, nil, ""},
	{"значок в строке звука", []string{".att-line .ic"}, []string{".ic", ".att .att-line .ic"}, []string{"width", "height", "flex", "color"}, nil, ""},
	{"название звука", []string{".att-name"}, []string{".att .att-name"}, []string{"font-size", "line-height", "flex", "min-width", "overflow", "text-overflow", "white-space"}, nil, ""},
	{"полоса хода", []string{".att-bar"}, []string{".att .att-bar"}, []string{"height", "border-radius", "background", "overflow"}, nil, ""},
	{"полоса хода: закраска", []string{".att-bar i"}, []string{".att .att-bar i"}, []string{"display", "height", "background", "width"}, nil, ""},
	{"длительность в голосовом", []string{".sz"}, []string{".crow .sz"}, []string{"font-size", "color"}, nil, ""},
	{"длительность в звуке", []string{".sz", ".att .sz"}, []string{".att .sz", ".att.audio .sz"}, []string{"font-size", "color", "line-height"}, nil, ""},

	// ——— Задокументированные отличия (план 46, A26) ———
	{"масштаб текста страницы", []string{"body"}, []string{".ph"}, []string{"font-size", "line-height"},
		map[string]string{"font-size": "15px", "line-height": "1.5"}, "приложение: 13.5 px / 1.45"},
	{"карточка записи", []string{".post"}, []string{".post"}, []string{"background", "border", "border-radius", "padding", "margin"},
		map[string]string{"border-radius": "16px", "padding": "14px 16px", "margin": "14px 12px"},
		"приложение: радиус 14 px, поля 14 px, отступ 12 px 14 px"},
	{"аватар", []string{".av"}, []string{".av"}, []string{"width", "height", "border-radius"},
		map[string]string{"width": "32px", "height": "32px"}, "приложение: 34 px"},
	{"буквы аватара", []string{".avl"}, []string{".av"}, []string{"width", "height", "border-radius", "font-size", "background"},
		map[string]string{"width": "32px", "height": "32px", "font-size": "14px", "background": "var(--c)"},
		"приложение: 34 px, шрифт 13.5 px, фон var(--rc, var(--c))"},
	{"время и дата записи", []string{".meta"}, []string{".pa .tm"}, []string{"color", "font-size"},
		map[string]string{"color": "var(--muted)", "font-size": "12.5px"}, "приложение: faint, 11.5 px"},
	{"текст записи", []string{".body"}, []string{".pt", ".mention-text"}, []string{"white-space", "overflow-wrap", "margin"},
		map[string]string{"overflow-wrap": "", "margin": "10px 0 0"},
		"приложение: overflow-wrap anywhere, отступ снизу 11 px"},
	{"фото и ролики записи", []string{".media img", ".media video"}, []string{".pic"}, []string{"border-radius", "border"},
		map[string]string{"border-radius": "12px", "border": ""}, "приложение: радиус 10 px, рамка 1 px; плитки против столбика — структурно"},
	{"отклики", []string{".reactions"}, []string{".rx .one"}, []string{"font-size"},
		map[string]string{"font-size": "13px"}, "приложение: 12.5 px; строка вместо чипсов — структурно"},
	{"комментарий", []string{".comment"}, []string{".cmt .g"}, []string{"font-size", "line-height"},
		map[string]string{"font-size": "14px", "line-height": ""}, "приложение: 12.5 px / 1.4; список внутри карточки против карточки .cm — структурно"},
	{"медиа комментария", []string{".cmedia img"}, []string{".cmedia"}, []string{"max-width"},
		map[string]string{"max-width": "240px"}, "приложение: потолка ширины нет, колонка с зазором 8 px"},
	{"файл записи и комментария", []string{".cfile"}, []string{".att .att-name"}, []string{"font-size"},
		map[string]string{"font-size": "14px"}, "приложение: 12.5 px; ссылка вместо карточки .att — структурно"},
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
// 0.07), убирает пробелы у запятых и скобок и приводит к нижнему регистру,
// чтобы сравнивать значения, а не формат записи. Пробелы между токенами
// («0 7px») остаются — они значимые.
func normalizeCSSValue(v string) string {
	v = cssSpace.ReplaceAllString(v, " ")
	v = strings.TrimSpace(v)
	v = cssLeadDot.ReplaceAllString(v, "${1}0.$2")
	v = strings.ReplaceAll(v, ", ", ",")
	v = strings.ReplaceAll(v, "( ", "(")
	v = strings.ReplaceAll(v, " )", ")")
	return strings.ToLower(v)
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

// TestArchiveStylesMatchApp сверяет стили архива с tokens.css и ui.css по
// списку свойств каждой записи таблицы. Расхождение падает, если его нет
// в known; исчезнувшее отличие — тоже падение: запись надо убрать.
func TestArchiveStylesMatchApp(t *testing.T) {
	read := func(name string) []cssRule {
		raw, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "lib", "styles", name))
		if err != nil {
			t.Fatalf("нет стилей приложения: %v", err)
		}
		return parseCSSRules(string(raw))
	}
	app := append(read("tokens.css"), read("ui.css")...)
	arch := parseCSSRules(baseCSS)
	for _, e := range archiveStyleParity {
		archDecls := mergeDecls(arch, e.arch)
		appDecls := mergeDecls(app, e.app)
		for _, prop := range e.props {
			got, inArch := archDecls[prop]
			want, inApp := appDecls[prop]
			k, documented := e.known[prop]
			switch {
			case inArch && got == want && documented && k != "":
				t.Errorf("%s: отличие по свойству %s исчезло — в архиве и приложении теперь %q, уберите запись из known (%s)", e.name, prop, got, e.note)
			case inArch && got == want:
				// равенство
			case inArch && documented && k == got:
				// задокументированное отличие
			case inArch && documented:
				t.Errorf("%s: отличие по свойству %s изменилось: в архиве %q, было записано %q, в приложении %q (%s)", e.name, prop, got, k, want, e.note)
			case inArch:
				t.Errorf("%s: %s — в архиве %q, в приложении %q. Если отличие намеренное — занесите его в known (%s)", e.name, prop, got, want, e.note)
			case !inApp && documented:
				// свойство есть только в архиве (или нигде) — так и записано
			case documented && k == "":
				// в архиве свойства нет — так и записано
			case documented:
				t.Errorf("%s: свойство %s пропало из архива, а было записано как %q, в приложении %q (%s)", e.name, prop, k, want, e.note)
			case !inApp:
				t.Errorf("%s: в приложении нет свойства %s, в архиве тоже; запись о свойстве лишняя (%s)", e.name, prop, e.note)
			default:
				t.Errorf("%s: в архиве нет свойства %s; в приложении %q. Если отличие намеренное — занесите его в known (%s)", e.name, prop, want, e.note)
			}
		}
	}
}
