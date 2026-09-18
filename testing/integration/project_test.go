//go:build integration

package integration_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/trustattic/trustattic-cli/testing/integration"
)

func TestProjectFlow(t *testing.T) {
	suite.Run(t, new(ProjectFlowSuite))
}

type ProjectFlowSuite struct {
	integration.BaseTestSuite
}

type projectListOutput struct {
	Data []struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"data"`
}

func (s *ProjectFlowSuite) TestListCreateList() {
	out, err := s.RunCLI("project", "list", "--output", "json")
	s.Require().NoError(err, out)

	var before projectListOutput
	s.Require().NoError(json.Unmarshal([]byte(out), &before))

	out, err = s.RunCLI("project", "create", "--name", "cli-created-project", "--output", "json")
	s.Require().NoError(err, out)

	out, err = s.RunCLI("project", "list", "--output", "json")
	s.Require().NoError(err, out)

	var after projectListOutput
	s.Require().NoError(json.Unmarshal([]byte(out), &after))

	found := false
	for _, p := range after.Data {
		if p.Name == "cli-created-project" {
			found = true
		}
	}
	s.Require().True(found, "created project not found in second 'project list': %s", out)
	s.Require().Greater(len(after.Data), len(before.Data))
}
