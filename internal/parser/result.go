package parser

type Context struct {
	DefaultLoadKG *float64
	ExpectedSets  int
	RequireLoad   bool
}

type Set struct {
	LoadKG *float64
	Reps   int
	RIR    *float64
	Pain   *int
}

type Result struct {
	SubstitutionAlias string
	Sets              []Set
	Warnings          []string
}
