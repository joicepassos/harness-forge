package domain

type Document struct {
	Path    string
	Content []byte
}
type Input struct {
	Project  string
	Rules    []Rule
	Commands []string
}
type Rule struct {
	ID, Description string
	Paths           []string
}
