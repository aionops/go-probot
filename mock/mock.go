package mock

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/h2non/gock"

	"github.com/aionops/go-probot"
)

type AppMock[GT probot.GitClientType] interface {
	GetServerOptions() probot.ServerOptions
	GetHTTPHandler() http.Handler
	GetSecretToken() []byte
}

func Send[GT probot.GitClientType](app AppMock[GT], e probot.WebhookEvent, v interface{}) error {
	serverOpts := app.GetServerOptions()

	gock.New(fmt.Sprintf("http://%s:%d", serverOpts.Address, serverOpts.Port)).
		Post(serverOpts.Path).EnableNetworking()

	body := &bytes.Buffer{}
	if err := json.NewEncoder(body).Encode(v); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s:%d%s", serverOpts.Address, serverOpts.Port, serverOpts.Path), body)
	if err != nil {
		return err
	}
	signature := getSignature(body.Bytes(), app.GetSecretToken())
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-GitHub-Event", e.Type())
	req.Header.Add("X-Gitlab-Event", e.Type())
	req.Header.Add("X-Hub-Signature-256", fmt.Sprintf("sha256=%s", signature))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status code: %d, body %s", resp.StatusCode, string(raw))
	}
	return nil
}

func getSignature(message, key []byte) string {
	hashFunc := sha256.New
	mac := hmac.New(hashFunc, key)
	mac.Write(message)
	return hex.EncodeToString(mac.Sum(nil))
}
