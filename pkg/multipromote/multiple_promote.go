package multipromote

import (
	"fmt"
	"github.com/jenkins-x-plugins/jx-application/pkg/applications"
	"github.com/jenkins-x-plugins/jx-promote/pkg/promote"
	jxcore "github.com/jenkins-x/jx-api/v4/pkg/apis/core/v4beta1"
	v1 "github.com/jenkins-x/jx-api/v4/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/templates"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/cli"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube/activities"
	"github.com/jenkins-x/jx-helpers/v3/pkg/requirements"
	"github.com/jenkins-x/jx-helpers/v3/pkg/stringhelpers"
	"github.com/jenkins-x/jx-helpers/v3/pkg/termcolor"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

const (
	optionEnvironment     = "env"
	optionFromEnvironment = "from-env"
)

// Options containers the CLI options
type Options struct {
	promote.Options
	Releases        []promote.ReleaseInfo
	FromEnvironment string
}

var (
	promoteMultipleLong = templates.LongDesc(`
		Promotes multiple applications from chosen environment to many permanent environments.
`)

	promoteExample = templates.Examples(`
		# Choose to promote applications from staging environment
        # to production environment
		jx promote multiple --from-env staging --env production
	`)
)

// NewCmdMultiplePromote creates the new command for: jx get prompt
func NewCmdMultiplePromote() *cobra.Command {
	options := &Options{}
	cmd := &cobra.Command{
		Use:     "multiple",
		Short:   "Promotes multiple applications from given environment",
		Long:    promoteMultipleLong,
		Example: promoteExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Environment, optionEnvironment, "e", "", "The Environment to promote to")
	cmd.Flags().StringVarP(&options.FromEnvironment, optionFromEnvironment, "", "", "The environment to promote from")
	cmd.MarkFlagRequired(optionEnvironment)
	cmd.MarkFlagRequired(optionFromEnvironment)
	return cmd
}

// Run implements this command
func (o *Options) Run() error {
	err := o.Validate()
	if err != nil {
		return errors.Wrapf(err, "failed to validate options")
	}
	list, err := applications.GetApplications(o.JXClient, o.KubeClient, o.Namespace, o.GitClient)
	if err != nil {
		return errors.Wrap(err, "fetching applications")
	}
	if len(list.Items) == 0 {
		log.Logger().Infof("No applications found")
		return nil
	}
	apps := o.GetEnvApplications(list, o.FromEnvironment)
	if len(apps) == 0 {
		return errors.New("No applications in given environment")
	}
	chosenApps, err := o.Input.SelectNames(apps, "Choose applications to promote", false, "please select an application")
	if err != nil {
		return errors.Wrapf(err, "failed to choose applications")
	}
	releases := []promote.ReleaseInfo{}
	for _, el := range chosenApps {
		parts := strings.Split(el, ":")
		appName := strings.TrimSpace(parts[0])
		version := strings.TrimSpace(parts[1])
		releases = append(releases, promote.ReleaseInfo{
			ReleaseName:     appName,
			FullAppName:     appName,
			Version:         version,
			PullRequestInfo: nil,
		})
	}
	o.Releases = releases
	ns := o.Namespace
	if ns == "" {
		return errors.Errorf("no namespace defined")
	}
	jxClient := o.JXClient
	if o.GitClient == nil {
		o.GitClient = cli.NewCLIClient("", o.CommandRunner)
	}

	err = o.DevEnvContext.LazyLoad(o.GitClient, o.JXClient, o.Namespace, o.Git(), o.Dir)
	if err != nil {
		return errors.Wrap(err, "failed to lazy load the EnvironmentContext")
	}

	if kube.IsInCluster() && !o.DisableGitConfig {
		err = o.InitGitConfigAndUser()
		if err != nil {
			return errors.Wrapf(err, "failed to init git")
		}
	}
	if o.HelmRepositoryURL == "" {
		o.HelmRepositoryURL, err = o.ResolveChartRepositoryURL()
		if err != nil {
			return errors.Wrapf(err, "failed to resolve helm repository URL")
		}
	}
	_, env, err := o.GetTargetNamespace(o.Namespace, o.Environment)
	if err != nil {
		return err
	}

	o.Activities = jxClient.JenkinsV1().PipelineActivities(ns)

	if o.ReleaseName == "" {
		o.ReleaseName = o.Application
	}
	_, err = o.Promote([]*jxcore.EnvironmentConfig{env}, true, o.NoPoll)
	if err != nil {
		return err
	}

	//o.ReleaseInfo = releaseInfo
	//if !o.NoPoll {
	//	err = o.WaitForPromotion(targetNS, env, releaseInfo)
	//	if err != nil {
	//		return err
	//	}
	//}
	return err

}

func (o *Options) GetEnvApplications(list applications.List, env string) []string {
	apps := []string{}
	for _, a := range list.Items {
		if val, ok := a.Environments[env]; ok {
			version := ""
			if len(val.Deployments) > 0 {
				version = val.Deployments[0].Version
			}
			app := fmt.Sprintf("%-36s: %s", a.Name(), version)
			apps = append(apps, app)
		}
	}
	return apps
}

func (o *Options) Promote(envs []*jxcore.EnvironmentConfig, warnIfAuto, noPoll bool) (*[]promote.ReleaseInfo, error) {
	if len(envs) == 0 {
		return nil, nil
	}
	var targetNamespaces []string
	for _, env := range envs {
		targetNS := promote.EnvironmentNamespace(env)
		if targetNS != "" && stringhelpers.StringArrayIndex(targetNamespaces, targetNS) < 0 {
			targetNamespaces = append(targetNamespaces, targetNS)
		}
	}
	for _, env := range envs {
		strategy := env.PromotionStrategy
		if string(strategy) == "" && env.Key == "staging" {
			// lets default the strategy based if its missing from the Environment
			strategy = v1.PromotionStrategyTypeAutomatic
		}
		draftPR := strategy != v1.PromotionStrategyTypeAutomatic
		targetNS := promote.EnvironmentNamespace(env)
		if targetNS == "" {
			return nil, fmt.Errorf("No namespace for environment %s", env.Key)
		}

		if warnIfAuto && env != nil && strategy == v1.PromotionStrategyTypeAutomatic && !o.BatchMode {
			log.Logger().Infof("%s", termcolor.ColorWarning(fmt.Sprintf("WARNING: The Environment %s is setup to promote automatically as part of the CI/CD Pipelines.\n", env.Key)))
			flag, err := o.Input.Confirm("Do you wish to promote anyway? :", false, "usually we do not manually promote to Auto promotion environments")
			if err != nil {
				return nil, errors.Wrapf(err, "failed to confirm promotion")
			}
			if !flag {
				return &o.Releases, nil
			}
		}

		jxClient := o.JXClient
		kubeClient := o.KubeClient
		promoteKey := o.CreatePromoteKey(env)
		if env != nil {
			if !envIsPermanent(env) {
				return nil, errors.Errorf("cannot promote to Environment which is not a permanent Environment")
			}

			sourceURL := requirements.EnvironmentGitURL(o.DevEnvContext.Requirements, env.Key)
			if sourceURL == "" && !env.RemoteCluster && o.DevEnvContext.DevEnv != nil {
				// lets default to the git repository of the dev environment as we are sharing the git repository across multiple namespaces
				sourceURL = o.DevEnvContext.DevEnv.Spec.Source.URL
			}
			if sourceURL != "" {
				err := o.PromoteViaPullRequest(envs, &o.Releases, draftPR)
				if err == nil {
					startPromotePR := func(a *v1.PipelineActivity, s *v1.PipelineActivityStep, ps *v1.PromoteActivityStep, p *v1.PromotePullRequestStep) error {
						activities.StartPromotionPullRequest(a, s, ps, p)
						pr := o.Releases[0].PullRequestInfo
						if pr != nil && pr.Link != "" {
							p.PullRequestURL = pr.Link
						}
						if noPoll {
							p.Status = v1.ActivityStatusTypeSucceeded
							ps.Status = v1.ActivityStatusTypeSucceeded
						}

						// if all steps are completed lets mark succeeded/failed
						activities.UpdateStatus(a, false, nil)
						return nil
					}
					err = promoteKey.OnPromotePullRequest(kubeClient, jxClient, o.Namespace, startPromotePR)
					if err != nil {
						log.Logger().Warnf("Failed to update PipelineActivity: %s", err)
					}
					// lets sleep a little before we try poll for the PR status
					time.Sleep(3 * time.Second)
				}
				return &o.Releases, err
			}
		}
	}
	return nil, errors.Errorf("no source repository URL available on  environment %s", o.Environment)
}

func envIsPermanent(env *jxcore.EnvironmentConfig) bool {
	return env.Key != "dev"
}
