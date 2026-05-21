package main

import (
	"context"

	"github.com/google/go-github/v48/github"
	"github.com/spf13/pflag"

	"github.com/aionops/go-probot"
)

func main() {
	app := probot.NewGitHubAPP()
	app.AddFlags(pflag.CommandLine)
	pflag.Parse()

	// Add a handler for events "issue_comment.created"
	app.On(probot.GitHub.IssueComment.Created).WithHandler(probot.GitHub.IssueComment.Handler(func(ctx probot.GitHubIssueCommentContext) {
		payload := ctx.Payload()
		ctx.Logger().Info("Get IssueComment event", "payload", payload)
		owner := payload.Repo.Owner.GetLogin()
		repo := payload.Repo.GetName()
		issueNumber := *payload.Issue.Number
		// If any error happen, the error message will be logged and sent as response
		ctx.Must(ctx.Client().Issues.CreateComment(ctx, owner, repo, issueNumber, &github.IssueComment{
			Body: github.String("Reply to this comment."),
		}))
	}))

	// Add a handler for multiple events
	app.On(
		probot.GitHub.PullRequest.Opened,      // pull_request.opened
		probot.GitHub.PullRequest.Edited,      // pull_request.edited
		probot.GitHub.PullRequest.Synchronize, // pull_request.synchronize
		probot.GitHub.PullRequest.Labeled,     // pull_request.labeled
		probot.GitHub.PullRequest.Assigned,    // pull_request.assigned
	).WithHandler(probot.GitHub.PullRequest.Handler(func(ctx probot.GitHubPullRequestContext) {
		payload := ctx.Payload()
		ctx.Logger().Info("Do something", "action", payload.GetAction(), "PullRequest labels", payload.PullRequest.Labels)
	}))

	// Add a handler for event "pull_request_review" with all action type
	app.On(probot.GitHub.PullRequestReview).WithHandler(probot.GitHub.PullRequestReview.Handler(func(ctx probot.GitHubPullRequestReviewContext) {
		payload := ctx.Payload()
		ctx.Logger().Info("Do something", "action", payload.GetAction(), "body", payload.Review.GetBody())
	}))

	if err := app.Run(context.Background()); err != nil {
		panic(err)
	}
}
