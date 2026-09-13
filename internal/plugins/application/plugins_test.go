package application

import (
	"context"
	"errors"
	"harnessforge/internal/plugins/domain"
	"strings"
	"testing"
	"time"
)

type catalog struct{ manifests []domain.Manifest }

func (c catalog) Discover(context.Context, string) ([]domain.Manifest, error) {
	return c.manifests, nil
}

type executor struct {
	response domain.Response
	err      error
	calls    int
}

func (e *executor) Execute(context.Context, string, domain.Manifest, domain.Request) (domain.Response, error) {
	e.calls++
	return e.response, e.err
}

func TestExecutionRequiresAuthorization(t *testing.T) {
	e := &executor{}
	_, err := NewPlugins(catalog{}, e).Execute(context.Background(), ".", "example", "analyzer", nil, false, time.Second)
	if err == nil || e.calls != 0 {
		t.Fatal(err)
	}
}

func TestIncompatiblePluginAndFailureAreSurfaced(t *testing.T) {
	bad := domain.Manifest{Name: "example", APIVersion: "v0", Capabilities: []string{"analyzer"}}
	_, err := NewPlugins(catalog{[]domain.Manifest{bad}}, &executor{}).Execute(context.Background(), ".", "example", "analyzer", nil, true, time.Second)
	if err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatal(err)
	}
	valid := domain.Manifest{Name: "example", APIVersion: domain.APIVersion, Capabilities: []string{"analyzer"}}
	_, err = NewPlugins(catalog{[]domain.Manifest{valid}}, &executor{err: errors.New("process exited")}).Execute(context.Background(), ".", "example", "analyzer", nil, true, time.Second)
	if err == nil || !strings.Contains(err.Error(), "process exited") {
		t.Fatal(err)
	}
}

func TestExampleCapabilityWorksThroughContract(t *testing.T) {
	m := domain.Manifest{Name: "word-count", APIVersion: domain.APIVersion, Capabilities: []string{"analyzer"}}
	e := &executor{response: domain.Response{APIVersion: domain.APIVersion, Output: map[string]any{"words": 2}}}
	result, err := NewPlugins(catalog{[]domain.Manifest{m}}, e).Execute(context.Background(), ".", "word-count", "analyzer", map[string]any{"text": "two words"}, true, time.Second)
	if err != nil || result.Output["words"] != 2 || e.calls != 1 {
		t.Fatal(result, err)
	}
}
