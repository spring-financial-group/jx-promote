package promote

import (
	"github.com/jenkins-x-plugins/jx-promote/pkg/environments"
	scmFake "github.com/jenkins-x/go-scm/scm/driver/fake"
	jxFake "github.com/jenkins-x/jx-api/v4/pkg/client/clientset/versioned/fake"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	kubeFake "k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestOptions_ValidateClients(t *testing.T) {
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
