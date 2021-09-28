package multipromote_test

import (
	"github.com/jenkins-x-plugins/jx-application/pkg/applications"
	"github.com/jenkins-x-plugins/jx-promote/pkg/multipromote"
	v1 "github.com/jenkins-x/jx-api/v4/pkg/apis/jenkins.io/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestGetEnvApplications(t *testing.T) {
	seekEnv := "production"
	list := applications.List{Items: []applications.Application{
		{&v1.SourceRepository{ObjectMeta: metav1.ObjectMeta{Name: "myStagingRepo"}}, map[string]applications.Environment{"staging": {}}},
		{&v1.SourceRepository{ObjectMeta: metav1.ObjectMeta{Name: "myDemoRepo"}}, map[string]applications.Environment{"demo": {}}},
		{&v1.SourceRepository{ObjectMeta: metav1.ObjectMeta{Name: "myProductionRepo"}}, map[string]applications.Environment{"production": {}}},
	}}
	expectedLength := 0
	options := multipromote.Options{}
	for _, a := range list.Items {
		if _, ok := a.Environments[seekEnv]; ok {
			expectedLength++
		}
	}
	appResult := options.GetEnvApplications(list, seekEnv)
	assert.Equal(t, expectedLength, len(appResult))
}
