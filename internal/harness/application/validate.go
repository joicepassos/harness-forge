package application

import "harnessforge/internal/harness/domain"

type Loader interface {
	Load(string) (domain.Harness, error)
}
type Validate struct{ loader Loader }

func NewValidate(loader Loader) *Validate { return &Validate{loader: loader} }
func (v *Validate) Execute(path string) error {
	h, err := v.loader.Load(path)
	if err != nil {
		return err
	}
	return h.Validate()
}
