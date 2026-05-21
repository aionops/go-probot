package probot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v48/github"
	"github.com/shurcooL/githubv4"
	"github.com/spf13/pflag"
	"golang.org/x/oauth2"

	"github.com/aionops/go-probot/web"
)

func NewGitHubAPP() App[GitHubClient] {
	return &githubApp{
		handlers:       make(map[string]Handler),
		clients:        make(map[int64]*github.Client),
		graphqlClients: make(map[int64]*githubv4.Client),
		serveMux:       http.NewServeMux(),
		metrics:        new(eventMetrics),
	}
}

type githubApp struct {
	handlers map[string]Handler
	clients  map[int64]*github.Client
	cliMutex sync.RWMutex

	graphqlClients map[int64]*githubv4.Client
	graphqlMutex   sync.RWMutex

	appID          int64
	tokenFile      string
	privateKeyFile string
	hmacTokenFile  string
	baseURL        string
	graphqlURL     string
	uploadURL      string
	serverOpts     ServerOptions
	serveMux       *http.ServeMux

	metrics *eventMetrics

	dataMutex  sync.RWMutex
	token      []byte
	hmacToken  []byte
	privateKey []byte
	loggerOptions
}

var _ App[GitHubClient] = &githubApp{}

func (app *githubApp) GetServerOptions() ServerOptions {
	return app.serverOpts
}

func (app *githubApp) GetHTTPHandler() http.Handler {
	return http.HandlerFunc(app.handle)
}

func (app *githubApp) GetSecretToken() []byte {
	app.dataMutex.RLock()
	defer app.dataMutex.RUnlock()
	return app.hmacToken
}

func (app *githubApp) AddFlags(flags *pflag.FlagSet) {
	app.serverOpts.AddFlags(flags)
	app.loggerOptions.AddFlags(flags)

	flags.Int64Var(&app.appID, "github.appid", 0, "github App id")
	flags.StringVar(&app.tokenFile, "github.token", "", "github App private-key file")
	flags.StringVar(&app.privateKeyFile, "github.private-key-file", "", "github App private-key file")
	flags.StringVar(&app.hmacTokenFile, "github.hmac-token-file", "", "github App hmac token file")
	flags.StringVar(&app.baseURL, "github.base-url", "https://api.github.com", "github base URL")
	flags.StringVar(&app.graphqlURL, "github.graphql-url", "https://api.github.com/graphql", "github graphql URL")
	flags.StringVar(&app.uploadURL, "github.upload-url", "https://upload.github.com", "github base URL")
}

func (app *githubApp) On(events ...WebhookEvent) handlerLoader {
	return handlerLoadFunc(func(h Handler) error {
		for _, event := range events {
			key := event.Type()
			if action := event.Action(); action != "" {
				key = key + "." + action
			}
			if _, ok := app.handlers[key]; ok {
				return fmt.Errorf("event type %s already exists", key)
			}
			app.handlers[key] = h
			app.metrics.add(key, event.Type())
		}
		return nil
	})
}

func (app *githubApp) Run(ctx context.Context) error {
	if err := app.initialize(); err != nil {
		return err
	}

	if app.serverOpts.Path == "" {
		app.serverOpts.Path = "/"
	}
	app.serveMux.HandleFunc(app.serverOpts.Path, app.handle)
	web.RegisterHandler(app.serveMux, app.metrics)
	server := &http.Server{Addr: fmt.Sprintf("%s:%d", app.serverOpts.Address, app.serverOpts.Port), Handler: app.serveMux}
	server.RegisterOnShutdown(app.shutdown)
	app.logger.Info("Kuilei hook is serving", "addr", server.Addr)
	return server.ListenAndServe()
}

func (app *githubApp) ServeMux() *http.ServeMux {
	return app.serveMux
}

func (app *githubApp) shutdown() {}

func (app *githubApp) initialize() error {
	app.dataMutex.Lock()
	defer app.dataMutex.Unlock()
	rawToken, err := os.ReadFile(app.hmacTokenFile)
	if err != nil {
		return fmt.Errorf("failed to read hmac token file, %w", err)
	}
	app.hmacToken = rawToken

	switch {
	case app.privateKeyFile != "":
		if app.appID == 0 {
			return errors.New("app-id must not be zero when private key file is provided")
		}
		rawPrivateKey, err := os.ReadFile(app.privateKeyFile)
		if err != nil {
			return fmt.Errorf("failed to read private key file, %w", err)
		}
		app.privateKey = rawPrivateKey
	case app.tokenFile != "":
		rawToken, err := os.ReadFile(app.tokenFile)
		if err != nil {
			return fmt.Errorf("failed to read token file, %w", err)
		}
		app.token = rawToken
	default:
		return errors.New("neither private key or token is provided")
	}
	return nil
}

//go:generate go run tools/codegen/main.go -t github_app_handle.go.tmpl -v github_events.yaml -o github_app_handle.go
func (app *githubApp) handle(w http.ResponseWriter, r *http.Request) {
	var handlerKey string
	defer func() {
		if e := recover(); e != nil {
			err, ok := e.(error)
			if ok {
				app.handleError(w, err, http.StatusBadRequest)
				app.logger.Error(
					err, "Failed handle event",
					"handler_key", handlerKey,
				)
			}
		}
	}()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	if r.Method != http.MethodPost {
		app.handleError(w, fmt.Errorf("method %s not found", r.Method), http.StatusNotFound)
		return
	}

	event := github.WebHookType(r)
	app.logger.Info("Handle event", "event", event)

	rawPayload, err := github.ValidatePayload(r, []byte(app.hmacToken))
	if err != nil {
		app.handleError(w, err, http.StatusBadRequest)
		return
	}

	defer func() {
		app.metrics.inc(handlerKey, event)
	}()

	handlerKey = app.handelEvent(ctx, w, r, event, rawPayload)
}

func (app *githubApp) handleError(w http.ResponseWriter, err error, status int) {
	w.WriteHeader(status)
	fmt.Fprint(w, err.Error())
}

func (app *githubApp) getClient(installID int64) (*github.Client, GitGraphQLClient, error) {
	cli, err := app.getGitHubClient(installID)
	if err != nil {
		return nil, nil, err
	}

	graphql, err := app.getGraphQLClient(installID)
	if err != nil {
		return nil, nil, err
	}
	return cli, graphql, nil
}

func (app *githubApp) getGitHubClient(installID int64) (*github.Client, error) {
	app.cliMutex.RLock()
	cli, ok := app.clients[installID]
	app.cliMutex.RUnlock()
	if ok {
		return cli, nil
	}

	if installID == 0 {
		// Not a github app, use oauth2 token
		var err error
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: strings.TrimSpace(string(app.token))})
		tc := oauth2.NewClient(context.TODO(), ts)
		cli, err = github.NewEnterpriseClient(app.baseURL, app.uploadURL, tc)
		if err != nil {
			return nil, err
		}
	} else {
		tr, err := ghinstallation.New(http.DefaultTransport, app.appID, installID, app.privateKey)
		if err != nil {
			return nil, err
		}
		cli, err = github.NewEnterpriseClient(app.baseURL, app.uploadURL, &http.Client{Transport: tr})
		if err != nil {
			return nil, err
		}
	}

	app.cliMutex.Lock()
	app.clients[installID] = cli
	app.cliMutex.Unlock()
	return cli, nil
}

func (app *githubApp) getGraphQLClient(installID int64) (GitGraphQLClient, error) {
	app.graphqlMutex.RLock()
	cli, ok := app.graphqlClients[installID]
	app.graphqlMutex.RUnlock()
	if ok {
		return cli, nil
	}

	if installID == 0 {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: strings.TrimSpace(string(app.token))})
		tc := oauth2.NewClient(context.TODO(), ts)
		cli = githubv4.NewEnterpriseClient(app.graphqlURL, &http.Client{Transport: tc.Transport})
	} else {
		tr, err := ghinstallation.New(http.DefaultTransport, app.appID, installID, app.privateKey)
		if err != nil {
			return nil, err
		}
		cli = githubv4.NewEnterpriseClient(app.graphqlURL, &http.Client{Transport: tr})
	}

	app.graphqlMutex.Lock()
	app.graphqlClients[installID] = cli
	app.graphqlMutex.Unlock()
	return cli, nil
}
