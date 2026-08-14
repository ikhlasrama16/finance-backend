package parser

type Registry struct{ parsers []Parser }

func NewRegistry() *Registry {
	return &Registry{parsers: []Parser{qrisParser{}, shopeePayParser{}, seaBankParser{}, genericParser{}}}
}
func (r *Registry) Parse(input Input) (*Result, error) {
	return r.ParseWithRules(input, nil)
}

type RuleMatcher func(Input) (*Result, bool, error)

func (r *Registry) ParseWithRules(input Input, matcher RuleMatcher) (*Result, error) {
	if r == nil {
		r = NewRegistry()
	}
	if r.parsers[0].CanParse(input) {
		return r.parsers[0].Parse(input)
	}
	if isPromotion(input) {
		return promotionResult(), nil
	}
	if matcher != nil {
		if result, matched, err := matcher(input); err != nil || matched {
			return result, err
		}
	}
	for _, p := range r.parsers[1:] {
		if p.CanParse(input) {
			if result, err := p.Parse(input); err != nil || result != nil {
				return result, err
			}
		}
	}
	return nil, nil
}

func Parse(input Input) (*Result, error) { return NewRegistry().Parse(input) }
