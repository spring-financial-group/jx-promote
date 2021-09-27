package multipromote

import (
	"fmt"
	"github.com/jenkins-x-plugins/jx-application/pkg/applications"
	"github.com/jenkins-x-plugins/jx-promote/pkg/promote"
	"github.com/jenkins-x/jx-api/v4/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/templates"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/v3/pkg/input"
	"github.com/jenkins-x/jx-helpers/v3/pkg/input/survey"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube/jxclient"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"strings"
)

const (
	optionEnvironment     = "env"
	optionFromEnvironment = "from-env"
)

// Options containers the CLI options
type Options struct {
	Args            []string
	Environment     string
	FromEnvironment string
	Namespace       string
	KubeClient      kubernetes.Interface
	JXClient        versioned.Interface
	Input           input.Interface
	GitClient       gitclient.Interface
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

// Validate validates settings
func (o *Options) Validate() error {
	if o.Input == nil {
		o.Input = survey.NewInput()
	}
	var err error
	o.KubeClient, o.Namespace, err = kube.LazyCreateKubeClientAndNamespace(o.KubeClient, o.Namespace)
	if err != nil {
		return errors.Wrapf(err, "failed to create the kube client")
	}
	o.JXClient, err = jxclient.LazyCreateJXClient(o.JXClient)
	if err != nil {
		return errors.Wrapf(err, "failed to create the jx client")
	}
	if o.Namespace == "" {
		return errors.Errorf("no namespace defined")
	}
	return nil
}

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
	apps := o.getEnvApplications(list, o.FromEnvironment)
	chosenApps, err := o.Input.SelectNames(apps, "Choose applications to promote", false, "please select an application")
	if err != nil {
		return errors.Wrapf(err, "failed to choose applications")
	}
	for i, el := range chosenApps {
		parts := strings.Split(el, ":")
		appName := parts[0]
		version := parts[1]
		promoteOptions := promote.Options{Environment: o.Environment, Application: appName, Version: version,
			Namespace: o.Namespace, KubeClient: o.KubeClient, JXClient: o.JXClient, Input: o.Input}
		log.Logger().Infof("Promoting %d out of %d applications.", i+1, len(chosenApps))
		err = promoteOptions.Run()
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *Options) getEnvApplications(list applications.List, env string) []string {
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
