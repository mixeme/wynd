package signing

// StepLink — действие в строке шага (например, «проверить» удостоверение
// сборки): окно открывает url, сама проверка ссылку не трогает.
type StepLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// Step — состояние этапа, совместимое с общим компонентом StepList окна.
type Step struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"` // pending, running, done, failed
	Note   string `json:"note,omitempty"`
	// Link — действие в строке шага; приходит вместе с итогом шага, а не
	// отдельным ответом после всей проверки.
	Link *StepLink `json:"link,omitempty"`
}

// Progress — ход проверки или подписи одного выпуска, без ключа и пароля.
type Progress struct {
	CheckID   string `json:"check_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Tag       string `json:"tag"`
	Phase     string `json:"phase"` // check, sign
	Steps     []Step `json:"steps"`
}

type progress struct {
	value  Progress
	report func(Progress)
}

func newProgress(tag, phase string, report func(Progress), steps ...Step) *progress {
	for i := range steps {
		steps[i].Status = "pending"
	}
	return &progress{value: Progress{Tag: tag, Phase: phase, Steps: steps}, report: report}
}

func (p *progress) update(id, status, note string) {
	p.change(id, func(s *Step) { s.Status, s.Note = status, note })
}

func (p *progress) start(id string) { p.update(id, "running", "") }

func (p *progress) done(id, note string) { p.update(id, "done", note) }

// doneLink отмечает шаг выполненным и даёт ему действие в строке шага:
// кнопка приходит тем же снимком, что и итог шага.
func (p *progress) doneLink(id, note, label, url string) {
	p.change(id, func(s *Step) {
		s.Status, s.Note = "done", note
		s.Link = &StepLink{Label: label, URL: url}
	})
}

func (p *progress) change(id string, f func(*Step)) {
	for i := range p.value.Steps {
		if p.value.Steps[i].ID == id {
			f(&p.value.Steps[i])
		}
	}
	if p.report != nil {
		snapshot := p.value
		snapshot.Steps = p.steps()
		p.report(snapshot)
	}
}

func (p *progress) fail() {
	for _, step := range p.value.Steps {
		if step.Status == "running" {
			p.update(step.ID, "failed", "")
			return
		}
	}
}

func (p *progress) steps() []Step { return append([]Step(nil), p.value.Steps...) }
