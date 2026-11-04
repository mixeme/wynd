package signing

// Step — состояние этапа, совместимое с общим компонентом StepList окна.
type Step struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"` // pending, running, done, failed
	Note   string `json:"note,omitempty"`
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
	for i := range p.value.Steps {
		if p.value.Steps[i].ID == id {
			p.value.Steps[i].Status = status
			p.value.Steps[i].Note = note
		}
	}
	if p.report != nil {
		snapshot := p.value
		snapshot.Steps = p.steps()
		p.report(snapshot)
	}
}

func (p *progress) start(id string) { p.update(id, "running", "") }

func (p *progress) done(id, note string) { p.update(id, "done", note) }

func (p *progress) fail() {
	for _, step := range p.value.Steps {
		if step.Status == "running" {
			p.update(step.ID, "failed", "")
			return
		}
	}
}

func (p *progress) steps() []Step { return append([]Step(nil), p.value.Steps...) }
