package promote

import (
	"bytes"
	"context"
	"github.com/jenkins-x-plugins/jx-promote/pkg/environments"
	"github.com/jenkins-x/go-scm/scm"
	scmFake "github.com/jenkins-x/go-scm/scm/driver/fake"
	jxFake "github.com/jenkins-x/jx-api/v4/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	kubeFake "k8s.io/client-go/kubernetes/fake"
	"os"
	"testing"
)

func TestOptions_validateClients(t *testing.T) {
	scmFakeClient, _ := scmFake.NewDefault()

	testCases := []struct {
		name          string
		testOptions   *Options
		expectedError error
	}{
		{
			name: "All_Clients",
			testOptions: &Options{
				EnvironmentPullRequestOptions: environments.EnvironmentPullRequestOptions{
					ScmClient: scmFakeClient,
				},
				JXClient:   jxFake.NewSimpleClientset(),
				KubeClient: kubeFake.NewSimpleClientset(),
			},
			expectedError: nil,
		},
		{
			name: "No_JX_Client",
			testOptions: &Options{
				EnvironmentPullRequestOptions: environments.EnvironmentPullRequestOptions{
					ScmClient: scmFakeClient,
				},
				JXClient:   nil,
				KubeClient: kubeFake.NewSimpleClientset(),
			},
			expectedError: errors.Errorf("no jx client"),
		},
		{
			name: "No_Kube_Client",
			testOptions: &Options{
				EnvironmentPullRequestOptions: environments.EnvironmentPullRequestOptions{
					ScmClient: scmFakeClient,
				},
				JXClient:   jxFake.NewSimpleClientset(),
				KubeClient: nil,
			},
			expectedError: errors.Errorf("no kube client"),
		},
		{
			name: "No_SCM_Client",
			testOptions: &Options{
				EnvironmentPullRequestOptions: environments.EnvironmentPullRequestOptions{
					ScmClient: nil,
				},
				JXClient:   jxFake.NewSimpleClientset(),
				KubeClient: kubeFake.NewSimpleClientset(),
			},
			expectedError: errors.Errorf("no ScmClient"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.testOptions.validateClients()
			if err != nil {
				assert.Errorf(t, err, testCase.expectedError.Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestOptions_prIsMergedWithSHA(t *testing.T) {
	type arguments struct {
		pr *scm.PullRequest
	}

	testCases := []struct {
		name         string
		args         arguments
		expectedBool bool
	}{
		{
			name: "Merged with SHA",
			args: arguments{
				pr: &scm.PullRequest{
					Merged:   true,
					MergeSha: "12345",
				},
			},
			expectedBool: true,
		},
		{
			name: "Merged without SHA",
			args: arguments{
				pr: &scm.PullRequest{
					Merged:   true,
					MergeSha: "",
				},
			},
			expectedBool: false,
		},
		{
			name: "Not Merged with SHA",
			args: arguments{
				pr: &scm.PullRequest{
					Merged:   false,
					MergeSha: "12345",
				},
			},
			expectedBool: false,
		},
		{
			name: "Not Merged without SHA",
			args: arguments{
				pr: &scm.PullRequest{
					Merged:   false,
					MergeSha: "",
				},
			},
			expectedBool: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expectedBool, prIsMergedWithSHA(testCase.args.pr))
		})
	}
}

func TestOptions_checkPullRequestStatus(t *testing.T) {
	type arguments struct {
		pr              *scm.PullRequest
		ctx             context.Context
		repo            scm.Repository
		prInfo          *scm.PullRequest
		logMergeFailure bool
	}

	testCases := []struct {
		name          string
		testOptions   Options
		args          arguments
		state         scm.State
		expectedError string
		expectedLog   string
	}{
		//{
		//	name:        "PullRequestLastCommitStatus_Fail",
		//	testOptions: Options{},
		//	args: arguments{
		//		pr:              &scm.PullRequest{},
		//		repo:            scm.Repository{},
		//		logMergeFailure: false,
		//	},
		//	expectedError: "",
		//	expectedLog:   "WARNING: Failed to query the Pull Request last commit status for  ref  no ScmClient\n",
		//},
		{
			name: "State is Pending",
			testOptions: Options{
				KubeClient: kubeFake.NewSimpleClientset(),
			},
			args: arguments{
				pr:              &scm.PullRequest{},
				repo:            scm.Repository{},
				logMergeFailure: false,
			},
			state:         scm.StatePending, // - Pending
			expectedError: "",
			expectedLog:   "The build for the Pull Request last commit is currently in progress.\n",
		},
		{
			name: "State is Success",
			testOptions: Options{
				KubeClient:         kubeFake.NewSimpleClientset(),
				NoMergePullRequest: true,
			},
			args: arguments{
				pr:              &scm.PullRequest{},
				repo:            scm.Repository{},
				logMergeFailure: false,
			},
			state:         scm.StateSuccess, // - Pending
			expectedError: "",
			expectedLog:   "",
		},
		{
			name: "State is Failure",
			testOptions: Options{
				KubeClient:         kubeFake.NewSimpleClientset(),
				NoMergePullRequest: true,
			},
			args: arguments{
				pr:              &scm.PullRequest{},
				repo:            scm.Repository{},
				logMergeFailure: false,
			},
			state:         scm.StateFailure, // - Failure
			expectedError: "pull request  last commit has status failure for ref ",
			expectedLog:   "",
		},
		{
			name: "State is Unknown",
			testOptions: Options{
				KubeClient:         kubeFake.NewSimpleClientset(),
				NoMergePullRequest: true,
			},
			args: arguments{
				pr:              &scm.PullRequest{},
				repo:            scm.Repository{},
				logMergeFailure: false,
			},
			state:         scm.StateUnknown, // - Unknown
			expectedError: "",
			expectedLog:   "got git provider status unknown from PR \n",
		},
	}

	for _, testCase := range testCases {
		// Build new fake SCM client for test
		scmFakeClient, scmFakeData := scmFake.NewDefault()
		scmFakeData.PullRequests[1] = testCase.args.pr
		testCase.testOptions.ScmClient = scmFakeClient
		scmFakeData.Statuses[""] = []*scm.Status{
			{
				State: testCase.state,
			},
		}

		t.Run(testCase.name, func(t *testing.T) {
			// Set logger to output to buffer to check logs
			var buf bytes.Buffer
			log.SetOutput(&buf)
			defer func() {
				log.SetOutput(os.Stderr)
			}()

			err := testCase.testOptions.checkPullRequestStatus(testCase.args.pr, testCase.args.ctx, testCase.args.repo, testCase.args.prInfo, testCase.args.logMergeFailure)
			if testCase.expectedError == "" {
				assert.Equal(t, testCase.expectedLog, buf.String())
				assert.NoError(t, err)
				return
			}
			assert.Equal(t, err.Error(), testCase.expectedError)
		})
	}
}
