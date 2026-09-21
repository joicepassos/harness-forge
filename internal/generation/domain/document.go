package domain

type Document struct {
	Path    string
	Content []byte
}
type Input struct {
	Project   string
	Summary   string
	Notes     string
	Documents []string
	Rules     []Rule
	Skills    []Skill
	Commands  []string
}
type Rule struct {
	ID, Description string
	Paths           []string
}
type Skill struct {
	ID, Description, Path string
}
