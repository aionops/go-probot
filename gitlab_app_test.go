package probot_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/h2non/gock"
	"github.com/onsi/gomega"
	"github.com/spf13/pflag"
	"github.com/xanzy/go-gitlab"

	"github.com/aionops/go-probot"
	"github.com/aionops/go-probot/mock"
)

func TestGitLabApp(t *testing.T) {
	gomega.RegisterTestingT(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f := gitlabFixture{}
	f.Start(ctx)
	defer f.Stop()

	app := createGitLabApp()
	go func() {
		if err := app.Run(ctx); err != nil {
			panic(err)
		}
	}()

	gomega.Eventually(func(g gomega.Gomega) {
		issueCommentEvent := gitlab.IssueCommentEvent{
			ObjectKind: "note",
			ProjectID:  1,
		}
		issueCommentEvent.ObjectAttributes.ID = 2
		issueCommentEvent.ObjectAttributes.Note = "hello"
		issueCommentEvent.ObjectAttributes.NoteableType = "Issue"
		issueCommentEvent.Issue.ID = 3
		g.Expect(mock.Send[probot.GitHubClient](
			app.(mock.AppMock[probot.GitHubClient]),
			probot.GitLab.IssueComment,
			issueCommentEvent,
		)).Should(gomega.Succeed())
	}, 5*time.Second, time.Second).Should(gomega.Succeed())
}

func createGitLabApp() probot.App[probot.GitLabClient] {
	flags := pflag.NewFlagSet("", pflag.PanicOnError)
	app := probot.NewGitLabAPP()
	app.AddFlags(flags)
	flags.Parse([]string{
		// TODO: add gitlab flags
		"--gitlab.token-file=examples/github/testdata/hmac_token",
		"--address=127.0.0.1",
		"--port=7771",
		"--path=/hook",
	})

	app.On(probot.GitLab.IssueComment).
		WithHandler(probot.GitLab.IssueComment.Handler(func(ctx probot.GitLabIssueCommentContext) {
			payload := ctx.Payload()
			ctx.Logger().Info("Get IssueComment event", "payload", payload, "content", payload.ObjectAttributes.Note)
			pid := payload.ProjectID
			issueID := payload.Issue.ID
			ctx.Must(ctx.Client().Notes.CreateIssueNote(
				pid, issueID, &gitlab.CreateIssueNoteOptions{
					Body: gitlab.String("Thanks!"),
				},
			))
		}))
	return app
}

type gitlabFixture struct {
}

func (f gitlabFixture) Start(ctx context.Context) {
	// TODO: Add more http fixture
	// Mock get token
	// gock.New("https://gitlab.com").
	// 	Get("/api/v4/").
	// 	Reply(404)

	gock.New("https://gitlab.com").
		Post("/api/v4/projects/1/issues/3/notes").
		AddMatcher(func(r1 *http.Request, r2 *gock.Request) (bool, error) {
			data := make(map[string]string)
			if err := json.NewDecoder(r1.Body).Decode(&data); err != nil {
				return false, err
			}
			return data["body"] == "Thanks!", nil
		}).
		Reply(200).JSON(gitlab.Note{})
}

func (gitlabFixture) Stop() {
	gock.Off()
}
