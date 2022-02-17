package promote

import (
	"bytes"
	"context"
	"github.com/jenkins-x-plugins/jx-promote/pkg/environments"
	"github.com/jenkins-x/go-scm/scm"
	scmFake "github.com/jenkins-x/go-scm/scm/driver/fake"
	jxcore "github.com/jenkins-x/jx-api/v4/pkg/apis/core/v4beta1"
	jxFake "github.com/jenkins-x/jx-api/v4/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/giturl"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube/activities"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	kubeFake "k8s.io/client-go/kubernetes/fake"
	"os"
	"testing"
	"time"
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
			assert.Equal(t, testCase.expectedLog, buf.String())
			if testCase.expectedError == "" {
				assert.NoError(t, err)
				return
			}
			assert.Equal(t, testCase.expectedError, err.Error())
		})
	}
}

func TestOptions_completePromotion(t *testing.T) {
	type arguments struct {
		ns         string
		env        *jxcore.EnvironmentConfig
		pr         *scm.PullRequest
		promoteKey *activities.PromoteStepActivityKey
	}

	testCases := []struct {
		name          string
		testOptions   Options
		args          arguments
		expectedError string
		expectedLog   string
	}{
		{
			name: "No Error returned",
			testOptions: Options{
				Namespace: "",
				JXClient:  jxFake.NewSimpleClientset(),
			},
			args: arguments{
				ns:  "TestNamespace",
				env: &jxcore.EnvironmentConfig{},
				pr:  &scm.PullRequest{MergeSha: "12345"},
				promoteKey: &activities.PromoteStepActivityKey{
					PipelineActivityKey: activities.PipelineActivityKey{
						Name: "TestPipeline",
					},
				},
			},
			expectedError: "",
			expectedLog:   "WARNING: No application name so cannot comment on issues that they are now in \n",
		},
		{
			name: "No wait",
			testOptions: Options{
				Namespace:        "",
				JXClient:         jxFake.NewSimpleClientset(),
				NoWaitAfterMerge: true,
			},
			args: arguments{
				ns:  "TestNamespace",
				env: &jxcore.EnvironmentConfig{},
				pr:  &scm.PullRequest{MergeSha: "12345"},
				promoteKey: &activities.PromoteStepActivityKey{
					PipelineActivityKey: activities.PipelineActivityKey{
						Name: "TestPipeline",
					},
				},
			},
			expectedError: "",
			expectedLog:   "Pull requests are merged, No wait on promotion to complete\n",
		},
		{
			name: "OnPromoteUpdate error",
			testOptions: Options{
				Namespace: "",
				JXClient:  jxFake.NewSimpleClientset(),
			},
			args: arguments{
				ns:  "TestNamespace",
				env: &jxcore.EnvironmentConfig{},
				pr:  &scm.PullRequest{MergeSha: "12345"},
				promoteKey: &activities.PromoteStepActivityKey{
					PipelineActivityKey: activities.PipelineActivityKey{
						Name: "TestPipeline",
						PullRefs: map[string]string{
							"test": "testRef",
						},
					},
				},
			},
			expectedError: "there was a problem reconciling batch build data: error parsing the current build number for PipelineActivity testpipeline: strconv.Atoi: parsing \"\": invalid syntax",
			expectedLog:   "Checking if batch build reconciling is needed\nNo past executions with the same lastCommitSha found - reconciliation not needed\nChecking if batch build reconciling is needed\n",
		},
		{
			name: "CommentOnIssues error",
			testOptions: Options{
				Namespace:   "",
				Application: "testApp",
				Version:     "testVersion",
				GitInfo:     &giturl.GitRepository{},
				JXClient:    jxFake.NewSimpleClientset(),
				KubeClient:  kubeFake.NewSimpleClientset(),
			},
			args: arguments{
				ns: "TestNamespace",
				env: &jxcore.EnvironmentConfig{
					Key:       "testKey",
					Namespace: "testNS",
				},
				pr: &scm.PullRequest{MergeSha: "12345"},
				promoteKey: &activities.PromoteStepActivityKey{
					PipelineActivityKey: activities.PipelineActivityKey{
						Name: "TestPipeline",
					},
				},
			},
			expectedError: "ingresses.extensions \"\" not found",
			expectedLog:   "WARNING: Could not find the service URL in namespace testNS for names testApp, , testNS-testApp\n",
		},
		//{
		//	name: "OnPromotePullRequest error",
		//	testOptions: Options{
		//		Namespace: "",
		//		JXClient:  jxFake.NewSimpleClientset(),
		//	},
		//	args: arguments{
		//		ns:  "TestNamespace",
		//		env: &jxcore.EnvironmentConfig{},
		//		pr:  &scm.PullRequest{MergeSha: "12345"},
		//		promoteKey: &activities.PromoteStepActivityKey{
		//			PipelineActivityKey: activities.PipelineActivityKey{
		//				Name: "TestPipeline",
		//			},
		//		},
		//	},
		//	expectedError: "",
		//	expectedLog:   "WARNING: No application name so cannot comment on issues that they are now in \n",
		//},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Set logger to output to buffer to check logs
			var buf bytes.Buffer
			log.SetOutput(&buf)
			defer func() {
				log.SetOutput(os.Stderr)
			}()

			err := testCase.testOptions.completePromotion(testCase.args.ns, testCase.args.env, testCase.args.pr, testCase.args.promoteKey)
			assert.Equal(t, testCase.expectedLog, buf.String())
			if testCase.expectedError == "" {
				assert.NoError(t, err)
				return
			}
			assert.Equal(t, testCase.expectedError, err.Error())
		})
	}
}

func TestOptions_checkMergePullRequest(t *testing.T) {
	type arguments struct {
		ctx             context.Context
		repo            scm.Repository
		pr              *scm.PullRequest
		prInfo          *scm.PullRequest
		logMergeFailure bool
	}

	testCases := []struct {
		name          string
		testOptions   Options
		args          arguments
		isTideRunning bool
		expectedLog   string
	}{
		//{
		//	name:        "ListStatus log",
		//	testOptions: Options{},
		//	args: arguments{
		//		ctx:             nil,
		//		repo:            scm.Repository{FullName: "TestRepo"},
		//		pr:              &scm.PullRequest{Head: scm.PullRequestBranch{Sha: "12345"}},
		//		prInfo:          &scm.PullRequest{Number: 1},
		//		logMergeFailure: false,
		//	},
		//	expectedLog: "",
		//},
		{
			name:        "NoMergePullRequest",
			testOptions: Options{NoMergePullRequest: true},
			args: arguments{
				ctx:             nil,
				repo:            scm.Repository{FullName: "TestRepo"},
				pr:              &scm.PullRequest{Head: scm.PullRequestBranch{Sha: "12345"}},
				prInfo:          &scm.PullRequest{Number: 0},
				logMergeFailure: false,
			},
			isTideRunning: false,
			expectedLog:   "",
		},
		{
			name:        "Merge Log",
			testOptions: Options{},
			args: arguments{
				ctx:             nil,
				repo:            scm.Repository{FullName: "TestRepo"},
				pr:              &scm.PullRequest{Head: scm.PullRequestBranch{Sha: "12345"}},
				prInfo:          &scm.PullRequest{Number: 0},
				logMergeFailure: false,
			},
			isTideRunning: false,
			expectedLog:   "WARNING: Failed to merge the Pull Request  due to pull request 0 not found maybe I don't have karma?\n",
		},
	}

	for _, testCase := range testCases {
		var tideLabel string
		if testCase.isTideRunning {
			tideLabel = "tide"
		}

		// Build new fake SCM client for test
		scmFakeClient, scmFakeData := scmFake.NewDefault()
		scmFakeData.PullRequests[1] = testCase.args.pr
		testCase.testOptions.ScmClient = scmFakeClient
		scmFakeData.Statuses["12345"] = []*scm.Status{
			{
				State: scm.StateSuccess,
				Label: tideLabel,
			},
		}

		t.Run(testCase.name, func(t *testing.T) {
			// Set logger to output to buffer to check logs
			var buf bytes.Buffer
			log.SetOutput(&buf)
			defer func() {
				log.SetOutput(os.Stderr)
			}()

			testCase.testOptions.checkMergePullRequest(testCase.args.ctx, testCase.args.repo, testCase.args.pr, testCase.args.prInfo, testCase.args.logMergeFailure)
			assert.Equal(t, testCase.expectedLog, buf.String())
		})
	}
}

func TestOptions_waitForGitOpsPullRequest(t *testing.T) {
	type arguments struct {
		ns          string
		env         *jxcore.EnvironmentConfig
		ReleaseInfo *ReleaseInfo
		end         time.Time
		duration    time.Duration
		promoteKey  *activities.PromoteStepActivityKey
	}

	testCases := []struct {
		name          string
		testOptions   Options
		args          arguments
		expectedLog   string
		expectedError string
	}{
		//{
		//	name:        "No PullRequestInfo",
		//	testOptions: Options{},
		//	args: arguments{
		//		ns:          "",
		//		env:         nil,
		//		ReleaseInfo: &ReleaseInfo{},
		//		end:         time.Time{},
		//		duration:    0,
		//		promoteKey:  nil,
		//	},
		//	expectedLog:   "",
		//	expectedError: "",
		//},
		//{
		//	name:        "No Clients",
		//	testOptions: Options{},
		//	args: arguments{
		//		ns:  "",
		//		env: nil,
		//		ReleaseInfo: &ReleaseInfo{
		//			ReleaseName:     "",
		//			FullAppName:     "",
		//			Version:         "",
		//			PullRequestInfo: &scm.PullRequest{},
		//		},
		//		end:        time.Time{},
		//		duration:   0,
		//		promoteKey: nil,
		//	},
		//	expectedLog:   "",
		//	expectedError: "no jx client",
		//},
		//{
		//	name: "Forced timeout",
		//	testOptions: Options{
		//		JXClient:   jxFake.NewSimpleClientset(),
		//		KubeClient: kubeFake.NewSimpleClientset(),
		//	},
		//	args: arguments{
		//		ReleaseInfo: &ReleaseInfo{
		//			PullRequestInfo: &scm.PullRequest{},
		//		},
		//		end:      time.Now().Add(-5 * time.Second),
		//		duration: -5 * time.Second,
		//	},
		//	expectedLog:   "",
		//	expectedError: "timed out waiting for pull request  to merge. Waited -5s",
		//},
		//{
		//	name: "PullRequest.Find error",
		//	testOptions: Options{
		//		JXClient:   jxFake.NewSimpleClientset(),
		//		KubeClient: kubeFake.NewSimpleClientset(),
		//	},
		//	args: arguments{
		//		ReleaseInfo: &ReleaseInfo{
		//			PullRequestInfo: &scm.PullRequest{},
		//		},
		//		end:      time.Now().Add(10 * time.Minute),
		//		duration: 10 * time.Minute,
		//	},
		//	expectedLog:   "",
		//	expectedError: "failed to find PR  0: Pull request number 0 does not exit",
		//},
		{
			name: "PullRequest.Find error",
			testOptions: Options{
				JXClient:   jxFake.NewSimpleClientset(),
				KubeClient: kubeFake.NewSimpleClientset(),
			},
			args: arguments{
				ReleaseInfo: &ReleaseInfo{
					PullRequestInfo: &scm.PullRequest{},
				},
				end:      time.Now().Add(10 * time.Minute),
				duration: 10 * time.Minute,
			},
			expectedLog:   "",
			expectedError: "failed to find PR  0: Pull request number 0 does not exit",
		},
	}

	for _, testCase := range testCases {
		// Build new fake SCM client for test
		scmFakeClient, _ := scmFake.NewDefault()
		//scmFakeData.PullRequests[1] = testCase.args.pr
		testCase.testOptions.ScmClient = scmFakeClient
		//scmFakeData.Statuses["12345"] = []*scm.Status{
		//	{
		//		State: scm.StateSuccess,
		//		Label: tideLabel,
		//	},
		//}

		t.Run(testCase.name, func(t *testing.T) {
			// Set logger to output to buffer to check logs
			var buf bytes.Buffer
			log.SetOutput(&buf)
			defer func() {
				log.SetOutput(os.Stderr)
			}()

			err := testCase.testOptions.waitForGitOpsPullRequest(testCase.args.ns, testCase.args.env, testCase.args.ReleaseInfo, testCase.args.end, testCase.args.duration, testCase.args.promoteKey)
			assert.Equal(t, testCase.expectedLog, buf.String())
			if testCase.expectedError == "" {
				assert.NoError(t, err)
				return
			}
			assert.Equal(t, testCase.expectedError, err.Error())
		})
	}
}
