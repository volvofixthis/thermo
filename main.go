package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"io"
	"net/http"
	"text/template"
	"time"

	"github.com/jfyne/live"
)

//go:embed static/*
var static embed.FS

type ReloadEngine struct {
	*live.HttpEngine
}

func NewReloadEngine(he *live.HttpEngine) *ReloadEngine {
	e := &ReloadEngine{
		he,
	}
	return e
}

func (e *ReloadEngine) Start() {
	go func() {
		time.Sleep(3 * time.Second)
		e.Broadcast("reload", nil)
	}()
}

type LiveReload struct {
	Revision int
}

type ThermoModel struct {
	Name        string
	Temperature float32
	Status      string
	LiveReload  LiveReload
	Time        string
}

func NewThermoModel(ctx context.Context, s live.Socket) *ThermoModel {
	m, ok := s.Assigns().(*ThermoModel)

	if !ok {
		m = &ThermoModel{
			Name:        live.Request(ctx).URL.Query().Get("name"),
			Temperature: 19.5,
			Status:      "",
			LiveReload:  LiveReload{1},
		}
	}
	return m
}

func thermoMount(ctx context.Context, s live.Socket) (interface{}, error) {
	fmt.Println("Mounting application")
	return NewThermoModel(ctx, s), nil
}

func tempUp(ctx context.Context, s live.Socket, p live.Params) (interface{}, error) {
	m := NewThermoModel(ctx, s)
	m.Temperature += 0.1
	return m, nil
}

func tempDown(ctx context.Context, s live.Socket, p live.Params) (interface{}, error) {
	m := NewThermoModel(ctx, s)
	m.Temperature -= 0.1
	return m, nil
}

func tempChange(ctx context.Context, s live.Socket, p live.Params) (interface{}, error) {
	m := NewThermoModel(ctx, s)
	t0 := m.Temperature
	m.Temperature += p.Float32("temperature")
	// Loacl
	// m.Status = fmt.Sprintf("Temperature change from %f to %f", t0, m.Temperature)
	// Shared
	s.Broadcast("status", fmt.Sprintf("%s: temperature change from %f to %f", m.Name, t0, m.Temperature))
	return m, nil
}

func saveEvent(ctx context.Context, s live.Socket, p live.Params) (interface{}, error) {
	m := NewThermoModel(ctx, s)
	message := p.String("message")
	s.Broadcast("status", m.Name+": "+message)
	return m, nil
}

func render(ctx context.Context, data *live.RenderContext) (io.Reader, error) {
	tmpl, err := template.New("thermo").Parse(`
		<!doctype html>
		<html  lang="en">
			<head>
				<!-- Required meta tags -->
				<meta charset="utf-8">
				<meta name="viewport" content="width=device-width, initial-scale=1">

				<link href="/static/css/main.css" rel="stylesheet">
				<!-- Bootstrap CSS -->
				<link href="https://cdn.jsdelivr.net/npm/bootstrap@5.0.2/dist/css/bootstrap.min.css" rel="stylesheet" integrity="sha384-EVSTQN3/azprG1Anm3QDgpJLIm9Nao0Yz1ztcQTwFspd3yD65VohhpuuCOmLASjC" crossorigin="anonymous">

				<title>Endless madness</title>
			</head>
			<body>
				<div class="container">
					<h4>User: {{.Assigns.Name}}</h4>
					<h2>Temperature: {{.Assigns.Temperature}}</h2>
					<div class="time-block">{{.Assigns.Time}}</div>
					<div>
					{{if gt .Assigns.Temperature 25.0 }}
						<hr4 class="temperature-warning">Warning temperature is too high(over 25.0)</h4>
					{{end}}
					</div>
					<div class="button-block">
						<button live-click="temp-up" live-window-keydown="temp-up" class="btn btn-success btn-sn" live-key="ArrowUp">+0.1C</button> -
						<button live-click="temp-down" live-window-keydown="temp-down" class="btn btn-success btn-sn" live-key="ArrowDown">-0.1C</button>
					</div>
					<div class="button-block">
						<button live-click="temp-change" live-value-temperature="2" class="btn btn-success btn-sn">+2C</button> -
						<button live-click="temp-change" live-value-temperature="-2" class="btn btn-success btn-sn">-2C</button>
					</div>
					<div class="send-block">
						<form live-submit="save" id="send-form" live-hook="submit">
							<input type="text" name="message">
							<input type="submit" value="submit" class="btn btn-success btn-sn">
						</form>
					</div>
					<div live-update="prepend">
						{{.Assigns.Status}}
					</div>
				</div>
				<div live-hook="reload" data-value="{{.Assigns.LiveReload.Revision}}" live-update="replace"></div>
				<script>
					window.Hooks = window.Hooks || {};
					window.Hooks['reload'] = {
						updated: function() {
							location.reload();
						}
					};
					window.Hooks['submit'] = {
						mounted: function() {
							this.el.addEventListener("submit", () => {
								this.el.querySelector("input").value = "";
							})
						}
					};
				</script>
				<script src="/live.js">
				<script src="https://cdn.jsdelivr.net/npm/bootstrap@5.0.2/dist/js/bootstrap.bundle.min.js" integrity="sha384-MrcW6ZMFYlzcLA8Nl+NtUVF0sA7MsXsP1UyJoMp4YLEuNSfAP+JcXn/tWtIaxVXM" crossorigin="anonymous"></script>
			</body>
		</html>
	`)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return &buf, nil
}

func main() {
	fmt.Println("Application started...")

	h := live.NewHandler()
	h.HandleRender(render)
	h.HandleMount(thermoMount)

	h.HandleEvent("temp-up", tempUp)
	h.HandleEvent("temp-down", tempDown)
	h.HandleEvent("temp-change", tempChange)
	h.HandleEvent("save", saveEvent)

	h.HandleSelf("reload", func(ctx context.Context, s live.Socket, d interface{}) (interface{}, error) {
		m := NewThermoModel(ctx, s)
		m.LiveReload.Revision += 1
		return m, nil
	})
	h.HandleSelf("status", func(ctx context.Context, s live.Socket, d interface{}) (interface{}, error) {
		m := NewThermoModel(ctx, s)
		m.Status = d.(string)
		return m, nil
	})
	h.HandleSelf("time", func(ctx context.Context, s live.Socket, d interface{}) (interface{}, error) {
		m := NewThermoModel(ctx, s)
		m.Time = d.(string)
		return m, nil
	})

	hh := live.NewHttpHandler(live.NewCookieStore("session-name", []byte("weak-secret")), h)
	go func() {
		for {
			hh.Broadcast("time", time.Now().Format(time.RFC1123))
			time.Sleep(1 * time.Second)
		}
	}()

	re := NewReloadEngine(hh)
	re.Start()

	mux := http.NewServeMux()

	mux.Handle("/thermostat", hh)
	mux.Handle("/live.js", live.Javascript{})
	staticServer := http.FileServer(http.FS(static))
	mux.Handle("/static/", staticServer)
	http.ListenAndServe(":8000", mux)
}
