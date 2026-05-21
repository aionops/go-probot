package probot_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-github/v48/github"
	"github.com/h2non/gock"
	"github.com/onsi/gomega"
	"github.com/spf13/pflag"

	"github.com/aionops/go-probot"
	"github.com/aionops/go-probot/mock"
)

func TestGitHubAPP(t *testing.T) {
	gomega.RegisterTestingT(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := githubFixture{installationID: 2}
	f.Start(ctx)
	defer f.Stop()

	app := createGitHubApp()
	go func() {
		if err := app.Run(ctx); err != nil {
			panic(err)
		}
	}()

	gomega.Eventually(func(g gomega.Gomega) {
		g.Expect(mock.Send[probot.GitHubClient](
			app.(mock.AppMock[probot.GitHubClient]),
			probot.GitHub.IssueComment.Created,
			github.IssueCommentEvent{
				Action:       probot.ToPointer("created"),
				Installation: &github.Installation{ID: probot.ToPointer(int64(f.installationID))},
				Repo: &github.Repository{
					Name:  probot.ToPointer("bar"),
					Owner: &github.User{Login: probot.ToPointer("foo")},
				},
				Issue: &github.Issue{Number: probot.ToPointer(1)},
			},
		)).Should(gomega.Succeed())
	}, 5*time.Second, time.Second).Should(gomega.Succeed())
}

func createGitHubApp() probot.App[probot.GitHubClient] {
	flags := pflag.NewFlagSet("", pflag.PanicOnError)
	app := probot.NewGitHubAPP()
	app.AddFlags(flags)
	flags.Parse([]string{
		"--github.hmac-token-file=examples/github/testdata/hmac_token",
		"--github.private-key-file=examples/github/testdata/tls.key",
		"--github.appid=1",
		"--address=127.0.0.1",
		"--port=7772",
		"--path=/hook",
	})

	app.On(probot.GitHub.IssueComment.Created, probot.GitHub.IssueComment.Edited).
		WithHandler(probot.GitHub.IssueComment.Handler(func(ctx probot.GitHubIssueCommentContext) {
			payload := ctx.Payload()
			ctx.Logger().Info("Get IssueComment event", "payload", payload)
			owner := *payload.Repo.Owner.Login
			repo := *payload.Repo.Name
			issueNumber := *payload.Issue.Number
			ctx.Must(ctx.Client().Issues.CreateComment(ctx, owner, repo, issueNumber, &github.IssueComment{
				Body: probot.ToPointer("Thanks!"),
			}))
		}))
	return app
}

type githubFixture struct {
	installationID int
}

func (f githubFixture) Start(ctx context.Context) {
	// Mock get token
	gock.New("https://api.github.com").
		Post(fmt.Sprintf("/app/installations/%d/access_tokens", f.installationID)).
		Reply(200).JSON(map[string]interface{}{
		"token": "test",
		"permissions": map[string]interface{}{
			"issues": "write",
		}})
	//
	gock.New("https://api.github.com").
		Post("/repos/foo/bar/issues/1/comments").
		AddMatcher(func(r1 *http.Request, r2 *gock.Request) (bool, error) {
			data := make(map[string]string)
			if err := json.NewDecoder(r1.Body).Decode(&data); err != nil {
				return false, err
			}
			return data["body"] == "Thanks!", nil
		}).
		Reply(200)
}

func (githubFixture) Stop() {
	gock.Off()
}
