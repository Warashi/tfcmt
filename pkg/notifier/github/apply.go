package github

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/tfcmt/pkg/notifier"
	"github.com/suzuki-shunsuke/tfcmt/pkg/terraform"
)

// Apply posts comment optimized for notifications
func (g *NotifyService) Apply(ctx context.Context, param *notifier.ParamExec) (int, error) {
	cfg := g.client.Config
	parser := g.client.Config.Parser
	template := g.client.Config.Template
	var errMsgs []string

	result := parser.Parse(param.CombinedOutput)
	result.ExitCode = param.ExitCode
	if result.HasParseError {
		template = g.client.Config.ParseErrorTemplate
	} else {
		if result.Error != nil {
			return result.ExitCode, result.Error
		}
		if result.Result == "" {
			return result.ExitCode, result.Error
		}
	}

	template.SetValue(terraform.CommonTemplate{
		Result:                 result.Result,
		ChangedResult:          result.ChangedResult,
		ChangeOutsideTerraform: result.OutsideTerraform,
		Warning:                result.Warning,
		HasDestroy:             result.HasDestroy,
		Link:                   cfg.CI,
		UseRawOutput:           cfg.UseRawOutput,
		Vars:                   cfg.Vars,
		Templates:              cfg.Templates,
		Stdout:                 param.Stdout,
		Stderr:                 param.Stderr,
		CombinedOutput:         param.CombinedOutput,
		ExitCode:               param.ExitCode,
		ErrorMessages:          errMsgs,
		CreatedResources:       result.CreatedResources,
		UpdatedResources:       result.UpdatedResources,
		DeletedResources:       result.DeletedResources,
		ReplacedResources:      result.ReplacedResources,
	})
	body, err := template.Execute()
	if err != nil {
		return result.ExitCode, err
	}
	if cfg.PR.Number == 0 {
		if prNumber, err := g.client.Commits.PRNumber(ctx, cfg.PR.Revision, PullRequestStateClosed); err == nil {
			cfg.PR.Number = prNumber
		}
	}

	logE := logrus.WithFields(logrus.Fields{
		"program": "tfcmt",
	})

	embeddedComment, err := getEmbeddedComment(cfg, param.CIName, false)
	if err != nil {
		return result.ExitCode, err
	}
	logE.WithFields(logrus.Fields{
		"comment": embeddedComment,
	}).Debug("embedded HTML comment")
	// embed HTML tag to hide old comments
	body += embeddedComment

	logE.Debug("create a comment")
	if err := g.client.Comment.Post(ctx, body, &PostOptions{
		Number:   cfg.PR.Number,
		Revision: cfg.PR.Revision,
	}); err != nil {
		return result.ExitCode, err
	}
	return result.ExitCode, nil
}
