package main

import (
	"context"
	"time"

	"github.com/google/go-github/v48/github"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"

	"github.com/aionops/go-probot"
	"github.com/aionops/go-probot/mock"
)

var _ = Describe("Test Probot Example", func() {
	It("Reply issue comments", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		flags := pflag.NewFlagSet("", pflag.PanicOnError)
		app := probot.NewGitHubAPP()
		app.AddFlags(flags)
		flags.Parse([]string{
			"--github.hmac-token-file=testdata/hmac_token",
			"--github.private-key-file=testdata/tls.key",
			"--github.appid=1",
			"--address=127.0.0.1",
			"--port=7771",
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

		go func() {
			Expect(app.Run(ctx)).Should(Succeed())
		}()

		Eventually(func(g Gomega) {
			g.Expect(mock.Send[probot.GitHubClient](
				app.(mock.AppMock[probot.GitHubClient]),
				probot.GitHub.IssueComment.Created,
				github.IssueCommentEvent{
					Action:       probot.ToPointer("created"),
					Installation: &github.Installation{ID: probot.ToPointer(int64(installationID))},
					Repo: &github.Repository{
						Name:  probot.ToPointer("bar"),
						Owner: &github.User{Login: probot.ToPointer("foo")},
					},
					Issue: &github.Issue{Number: probot.ToPointer(1)},
				},
			)).Should(Succeed())
		}, 5*time.Second, time.Second).Should(Succeed())
	})
})
