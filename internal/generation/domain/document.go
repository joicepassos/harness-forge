package domain

type Document struct {
	Path    string
	Content []byte
}
type Input struct {
	Project      string
	Summary      string
	Notes        string
	Documents    []string
	Architecture []string
	Rules        []Rule
	Skills       []Skill
	Gates        []QualityGate
	Policies     []Policy
	Commands     []string
}
type Rule struct {
	ID, Description string
	Paths           []string
}
type Skill struct {
	ID, Description, Path string
}
type QualityGate struct {
	ID, Command, Workspace string
	Workspaces             []string
}

type Policy struct{ ID, Description, Capability, Executor string }
