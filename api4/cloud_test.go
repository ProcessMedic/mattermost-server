// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v6/einterfaces/mocks"
	"github.com/mattermost/mattermost-server/v6/model"
)

func Test_getCloudLimits(t *testing.T) {
	t.Run("feature flag off returns empty limits", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		os.Setenv("MM_FEATUREFLAGS_CLOUDFREE", "false")
		defer os.Unsetenv("MM_FEATUREFLAGS_CLOUDFREE")
		th.App.ReloadConfig()

		th.App.Srv().SetLicense(model.NewTestLicense("cloud"))
		th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)

		limits, r, err := th.Client.GetProductLimits()
		require.NoError(t, err)
		require.Equal(t, limits, &model.ProductLimits{})
		require.Equal(t, http.StatusOK, r.StatusCode, "Expected 200 OK")
	})

	t.Run("no license returns not implemented", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		os.Setenv("MM_FEATUREFLAGS_CLOUDFREE", "true")
		defer os.Unsetenv("MM_FEATUREFLAGS_CLOUDFREE")
		th.App.ReloadConfig()

		th.App.Srv().RemoveLicense()

		th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)

		limits, r, err := th.Client.GetProductLimits()
		require.Error(t, err)
		require.Nil(t, limits)
		require.Equal(t, http.StatusNotImplemented, r.StatusCode, "Expected 501 Not Implemented")
	})

	t.Run("non cloud license returns not implemented", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		os.Setenv("MM_FEATUREFLAGS_CLOUDFREE", "true")
		defer os.Unsetenv("MM_FEATUREFLAGS_CLOUDFREE")
		th.App.ReloadConfig()

		th.App.Srv().SetLicense(model.NewTestLicense())

		th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)

		limits, r, err := th.Client.GetProductLimits()
		require.Error(t, err)
		require.Nil(t, limits)
		require.Equal(t, http.StatusNotImplemented, r.StatusCode, "Expected 501 Not Implemented")
	})

	t.Run("error fetching limits returns internal server error", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		os.Setenv("MM_FEATUREFLAGS_CLOUDFREE", "true")
		defer os.Unsetenv("MM_FEATUREFLAGS_CLOUDFREE")
		th.App.ReloadConfig()
		th.App.Srv().SetLicense(model.NewTestLicense("cloud"))

		cloud := &mocks.CloudInterface{}
		cloud.Mock.On("GetCloudLimits", mock.Anything).Return(nil, errors.New("Unable to get limits"))

		cloudImpl := th.App.Srv().Cloud
		defer func() {
			th.App.Srv().Cloud = cloudImpl
		}()
		th.App.Srv().Cloud = cloud

		th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)

		limits, r, err := th.Client.GetProductLimits()
		require.Error(t, err)
		require.Nil(t, limits)
		require.Equal(t, http.StatusInternalServerError, r.StatusCode, "Expected 500 Internal Server Error")
	})

	t.Run("unauthenticated users can not access", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		th.Client.Logout()

		limits, r, err := th.Client.GetProductLimits()
		require.Error(t, err)
		require.Nil(t, limits)
		require.Equal(t, http.StatusUnauthorized, r.StatusCode, "Expected 401 Unauthorized")
	})

	t.Run("good request with cloud server and feature flag returns response", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		os.Setenv("MM_FEATUREFLAGS_CLOUDFREE", "true")
		defer os.Unsetenv("MM_FEATUREFLAGS_CLOUDFREE")
		th.App.ReloadConfig()
		th.App.Srv().SetLicense(model.NewTestLicense("cloud"))

		cloud := &mocks.CloudInterface{}
		ten := 10
		mockLimits := &model.ProductLimits{
			Messages: &model.MessagesLimits{
				History: &ten,
			},
		}
		cloud.Mock.On("GetCloudLimits", mock.Anything).Return(mockLimits, nil)

		cloudImpl := th.App.Srv().Cloud
		defer func() {
			th.App.Srv().Cloud = cloudImpl
		}()
		th.App.Srv().Cloud = cloud

		th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)

		limits, r, err := th.Client.GetProductLimits()
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, r.StatusCode, "Expected 200 OK")
		require.Equal(t, mockLimits, limits)
		require.Equal(t, *mockLimits.Messages.History, *limits.Messages.History)
	})
}

func Test_requestTrial(t *testing.T) {
	subscription := &model.Subscription{
		ID:         "MySubscriptionID",
		CustomerID: "MyCustomer",
		ProductID:  "SomeProductId",
		AddOns:     []string{},
		StartAt:    1000000000,
		EndAt:      2000000000,
		CreateAt:   1000000000,
		Seats:      10,
		DNS:        "some.dns.server",
		IsPaidTier: "false",
	}

	t.Run("NON Admin users are UNABLE to request the trial", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		os.Setenv("MM_FEATUREFLAGS_CLOUDFREE", "true")
		defer os.Unsetenv("MM_FEATUREFLAGS_CLOUDFREE")
		th.App.ReloadConfig()

		th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)

		th.App.Srv().SetLicense(model.NewTestLicense("cloud"))

		cloud := mocks.CloudInterface{}

		cloud.Mock.On("GetSubscription", mock.Anything).Return(subscription, nil)
		cloud.Mock.On("RequestCloudTrial", mock.Anything, mock.Anything).Return(subscription, nil)

		cloudImpl := th.App.Srv().Cloud
		defer func() {
			th.App.Srv().Cloud = cloudImpl
		}()
		th.App.Srv().Cloud = &cloud

		subscriptionChanged, r, err := th.Client.RequestCloudTrial()
		t.Logf("\n\nresp %#v, \n\n r: %v\n\n, err: %v\n\n", subscriptionChanged, r, err)
		require.Error(t, err)
		require.Nil(t, subscriptionChanged)
		require.Equal(t, http.StatusForbidden, r.StatusCode, "403 Forbidden")
	})

	t.Run("cloudFree feature flag FALSE and Admin user are UNABLE to request the trial", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		os.Setenv("MM_FEATUREFLAGS_CLOUDFREE", "false")
		defer os.Unsetenv("MM_FEATUREFLAGS_CLOUDFREE")
		th.App.ReloadConfig()

		th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)

		th.App.Srv().SetLicense(model.NewTestLicense("cloud"))

		cloud := mocks.CloudInterface{}

		cloud.Mock.On("GetSubscription", mock.Anything).Return(subscription, nil)
		cloud.Mock.On("RequestCloudTrial", mock.Anything, mock.Anything).Return(subscription, nil)

		cloudImpl := th.App.Srv().Cloud
		defer func() {
			th.App.Srv().Cloud = cloudImpl
		}()
		th.App.Srv().Cloud = &cloud

		subscriptionChanged, r, err := th.SystemAdminClient.RequestCloudTrial()

		require.Error(t, err)
		require.Nil(t, subscriptionChanged)
		require.Equal(t, http.StatusInternalServerError, r.StatusCode, "Expected 500 Internal Server Error")
	})

	t.Run("cloudFree feature flag TRUE and ADMIN user are ABLE to request the trial", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		os.Setenv("MM_FEATUREFLAGS_CLOUDFREE", "true")
		defer os.Unsetenv("MM_FEATUREFLAGS_CLOUDFREE")
		th.App.ReloadConfig()

		th.Client.Login(th.BasicUser.Email, th.BasicUser.Password)

		th.App.Srv().SetLicense(model.NewTestLicense("cloud"))

		cloud := mocks.CloudInterface{}

		cloud.Mock.On("GetSubscription", mock.Anything).Return(subscription, nil)
		cloud.Mock.On("RequestCloudTrial", mock.Anything, mock.Anything).Return(subscription, nil)

		cloudImpl := th.App.Srv().Cloud
		defer func() {
			th.App.Srv().Cloud = cloudImpl
		}()
		th.App.Srv().Cloud = &cloud

		subscriptionChanged, r, err := th.SystemAdminClient.RequestCloudTrial()

		require.NoError(t, err)
		require.Equal(t, subscriptionChanged, subscription)
		require.Equal(t, http.StatusOK, r.StatusCode, "Status OK")
	})
}

func TestNotifyAdminToUpgrade(t *testing.T) {
	t.Run("user can only notify admin once", func(t *testing.T) {
		th := Setup(t).InitBasic().InitLogin()
		defer th.TearDown()

		statusCode := th.Client.NotifyAdmin(&model.NotifyAdminToUpgradeRequest{
			CurrentTeamId: th.BasicTeam.Id,
			CurrentUserId: th.BasicUser.Id,
		})

		bot, appErr := th.App.GetSystemBot()
		require.Nil(t, appErr)

		channel, err := th.App.Srv().Store.Channel().GetByName("", model.GetDMNameFromIds(bot.UserId, th.SystemAdminUser.Id), false)
		require.NoError(t, err)

		postList, err := th.App.Srv().Store.Post().GetPosts(model.GetPostsOptions{ChannelId: channel.Id, Page: 0, PerPage: 1}, false)
		require.NoError(t, err)

		require.Equal(t, len(postList.Order), 1)

		post := postList.Posts[postList.Order[0]]

		require.Equal(t, fmt.Sprintf("%sup_notification", model.PostCustomTypePrefix), post.Type)
		require.Equal(t, bot.UserId, post.UserId)
		require.Equal(t, fmt.Sprintf("A member of %s has notified you to upgrade this workspace before the trial ends.", th.BasicTeam.Name), post.Message)

		require.Equal(t, http.StatusOK, statusCode)

		// second time trying to call notify endpoint by same user is forbidden
		statusCode = th.Client.NotifyAdmin(&model.NotifyAdminToUpgradeRequest{
			CurrentTeamId: th.BasicTeam.Id,
			CurrentUserId: th.BasicUser.Id,
		})

		require.Equal(t, http.StatusForbidden, statusCode)
	})
}
