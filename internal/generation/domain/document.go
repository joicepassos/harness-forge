package domain

type Document struct {
	Path    string
	Content []byte
}
type Input struct {
	Project string
	Rules   []Rule
	Skills  []Skill
	Gates   []QualityGate
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
