package skill

type ActivationDecision string

const (
	ActivationAutoLoad    ActivationDecision = "auto_load"
	ActivationModelSelect ActivationDecision = "model_select"
	ActivationIgnored     ActivationDecision = "ignored"
)

type Activation struct {
	Match
	Decision ActivationDecision
}

type ActivatedSkill struct {
	Match
	Body string
}

type Resolver struct {
	AutoLoadThreshold    int
	ModelSelectThreshold int
	AutoLoadConfidence   float64
	MaxAutoLoaded        int
	AmbiguityScoreDelta  int
}

func (r Resolver) Resolve(matches []Match) []Activation {
	r = r.withDefaults()
	result := make([]Activation, 0, len(matches))
	ambiguousTop := len(matches) > 1 &&
		matches[0].Score >= r.ModelSelectThreshold &&
		matches[1].Score >= r.ModelSelectThreshold &&
		matches[0].Score-matches[1].Score <= r.AmbiguityScoreDelta
	autoLoaded := 0

	for _, match := range matches {
		decision := ActivationIgnored
		if match.Score >= r.ModelSelectThreshold {
			decision = ActivationModelSelect
		}
		if !ambiguousTop &&
			match.Score >= r.AutoLoadThreshold &&
			match.Confidence >= r.AutoLoadConfidence &&
			autoLoaded < r.MaxAutoLoaded {
			decision = ActivationAutoLoad
			autoLoaded++
		}
		result = append(result, Activation{
			Match:    match,
			Decision: decision,
		})
	}
	return result
}

func (r Resolver) withDefaults() Resolver {
	if r.ModelSelectThreshold <= 0 {
		r.ModelSelectThreshold = DefaultCandidateMinScore
	}
	if r.AutoLoadThreshold <= 0 {
		r.AutoLoadThreshold = 16
	}
	if r.AutoLoadConfidence <= 0 {
		r.AutoLoadConfidence = 0.75
	}
	if r.MaxAutoLoaded <= 0 {
		r.MaxAutoLoaded = 1
	}
	if r.AmbiguityScoreDelta <= 0 {
		r.AmbiguityScoreDelta = 3
	}
	return r
}
