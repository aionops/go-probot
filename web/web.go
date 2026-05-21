package web

import (
	"embed"
	"io/fs"
	"net/http"

	restfulspec "github.com/emicklei/go-restful-openapi/v2"
	"github.com/emicklei/go-restful/v3"
	"github.com/go-openapi/spec"

	"github.com/aionops/go-probot/web/backend"
)

//go:embed dist/*
var static embed.FS

func Static() fs.FS {
	f, err := fs.Sub(static, "dist")
	if err != nil {
		panic(err)
	}
	return f
}

func StaticHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawFile, _ := static.ReadFile("dist/index.html")
		w.Write(rawFile)
	})
}

func RegisterHandler(
	mux *http.ServeMux,
	event backend.EventSource,
) {
	mux.Handle("/dashboard/", StaticHandler())
	mux.Handle("/assets/", http.FileServer(http.FS(Static())))

	ws := backend.WebService(event)
	c := restful.NewContainer()
	c.ServeMux = mux
	c.Add(ws)

	config := restfulspec.Config{
		WebServices:                   c.RegisteredWebServices(), // you control what services are visible
		APIPath:                       "/api/apidocs.json",
		PostBuildSwaggerObjectHandler: enrichSwaggerObject,
	}
	c.Add(restfulspec.NewOpenAPIService(config))
}

func enrichSwaggerObject(swo *spec.Swagger) {
	swo.Info = &spec.Info{
		InfoProps: spec.InfoProps{
			Title:       "ModelRegistry Service",
			Description: "Resource for managing model registry",
			Version:     "0.0.1",
		},
	}
	swo.Tags = []spec.Tag{{TagProps: spec.TagProps{
		Name:        "go-probot",
		Description: "Dashboard backedn API"}}}
}
