package multipromote

import (
	"fmt"
	"github.com/jenkins-x-plugins/jx-promote/pkg/promote"
	"github.com/jenkins-x-plugins/jx-promote/pkg/promoteconfig"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules"
	"github.com/jenkins-x-plugins/jx-promote/pkg/rules/factory"
	"github.com/jenkins-x/go-scm/scm"
	jxcore "github.com/jenkins-x/jx-api/v4/pkg/apis/core/v4beta1"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/gitconfig"
	"github.com/jenkins-x/jx-helpers/v3/pkg/requirements"
	"github.com/pkg/errors"
)

func (o *Options) PromoteViaPullRequest(envs []*jxcore.EnvironmentConfig, releaseInfo *[]promote.ReleaseInfo, draftPR bool) error {
	source := "promote-multiple-apps-to-" + envs[0].Key
	var labels []*scm.Label

	for _, env := range envs {
		envName := env.Key
		source += "-" + envName
		labels = append(labels, &scm.Label{
			Name:        "env/" + envName,
			Description: envName,
		})
	}

	comment := fmt.Sprintf("chore: promote multiple apps") + "\n\nthis commit will trigger a pipeline to [generate the actual kubernetes resources to perform the promotion](https://jenkins-x.io/docs/v3/about/how-it-works/#promotion) which will create a second commit on this Pull Request before it can merge"
	details := scm.PullRequest{
		Source: source,
		Title:  fmt.Sprintf("chore: promote multiple apps to %s", envs[0].Key),
		Body:   comment,
		Draft:  draftPR,
		Labels: labels,
	}

	if draftPR {
		details.Labels = append(details.Labels, &scm.Label{
			Name:        "do-not-merge/hold",
			Description: "do not merge yet",
		})
	}

	o.EnvironmentPullRequestOptions.CommitTitle = details.Title
	o.EnvironmentPullRequestOptions.CommitMessage = details.Body
	envDir := ""
	if o.CloneDir != "" {
		envDir = o.CloneDir
	}

	o.Function = func() error {
		dir := o.OutDir

		for _, env := range envs {
			promoteNS := promote.EnvironmentNamespace(env)
			promoteConfig, _, err := promoteconfig.Discover(dir, promoteNS)
			if err != nil {
				return errors.Wrapf(err, "failed to discover the PromoteConfig in dir %s", dir)
			}

			// lets check if we need the apps git URL
			if promoteConfig.Spec.FileRule != nil || promoteConfig.Spec.KptRule != nil {
				if o.AppGitURL == "" {
					_, gitConf, err := gitclient.FindGitConfigDir("")
					if err != nil {
						return errors.Wrapf(err, "failed to find git config dir")
					}
					o.AppGitURL, err = gitconfig.DiscoverUpstreamGitURL(gitConf, true)
					if err != nil {
						return errors.Wrapf(err, "failed to discover application git URL")
					}
					if o.AppGitURL == "" {
						return errors.Errorf("could not to discover application git URL")
					}
				}
			}

			for _, release := range *releaseInfo {
				newRule := &rules.PromoteRule{
					TemplateContext: rules.TemplateContext{
						GitURL:            "",
						Version:           release.Version,
						AppName:           release.ReleaseName,
						ChartAlias:        o.Alias,
						Namespace:         o.Namespace,
						HelmRepositoryURL: o.HelmRepositoryURL,
						ReleaseName:       o.ReleaseName,
					},
					Dir:           dir,
					Config:        *promoteConfig,
					DevEnvContext: &o.DevEnvContext,
				}
				newRule.TemplateContext.GitURL = o.AppGitURL

				fn := factory.NewFunction(newRule)
				if fn == nil {
					return errors.Errorf("could not create rule function ")
				}
				err = fn(newRule)
				if err != nil {
					return errors.Wrapf(err, "failed to promote to %s", env.Key)
				}

			}

		}
		return nil
	}

	env := envs[0]
	gitURL := requirements.EnvironmentGitURL(o.DevEnvContext.Requirements, env.Key)
	if gitURL == "" {
		if env.RemoteCluster {
			return errors.Errorf("no git URL for remote cluster %s", env.Key)
		}

		// lets default to the git repository for the dev environment for local clusters
		gitURL = requirements.EnvironmentGitURL(o.DevEnvContext.Requirements, "dev")
		if gitURL == "" {
			return errors.Errorf("no git URL for dev environment")
		}
	}
	_, err := o.Create(gitURL, envDir, &details, false)
	return err
}
