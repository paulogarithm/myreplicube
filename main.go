package main

import (
	// "fmt"
	"fmt"
	"log"
	"time"

	"github.com/g3n/engine/app"
	"github.com/g3n/engine/camera"
	"github.com/g3n/engine/core"
	"github.com/g3n/engine/geometry"
	"github.com/g3n/engine/gls"
	"github.com/g3n/engine/graphic"
	"github.com/g3n/engine/light"
	"github.com/g3n/engine/material"
	"github.com/g3n/engine/math32"
	"github.com/g3n/engine/renderer"
	"github.com/g3n/engine/window"

	// "github.com/g3n/engine/experimental/collision"

	"github.com/Shopify/go-lua"
	"github.com/fsnotify/fsnotify"
)

// the replicube app part

type ReplicubeApp struct {
	G3NApp *app.Application
	Scene *core.Node
	Elements map[string]core.INode
	Materials map[string]material.IMaterial
	Renderer *renderer.Renderer
	LuaState *lua.State
}

func (a ReplicubeApp) Register(name string, node core.INode) {
	a.Scene.Add(node)
	a.Elements[name] = node
}

// the setup functions

func setupInstances(app *ReplicubeApp) {
	// create the cube stuff
	geom := geometry.NewCube(1)
	mat := material.NewStandard(math32.NewColor("LightGray"))
	app.Materials["cubeMaterial"] = mat

	// create the cube mesh
	mesh := graphic.NewMesh(geom, mat)
	meshPos := mesh.Position()
	app.Register("cubeMesh", mesh)

	// create a camera
	cam := camera.New(1)
	cam.SetPosition(0, 2, 3)
	cam.LookAt(&meshPos, &math32.Vector3{X:0, Y:1, Z:0})
	app.Register("cam", cam)
	
	// create lights
	app.Register("ambientLight", light.NewAmbient(&math32.Color{R:1.0, G:1.0, B:1.0}, 0.8))
	pointLight := light.NewPoint(&math32.Color{R:1, G:1, B:1}, 5.0)
	pointLight.SetPosition(1, 0, 2)
	app.Register("pointLight", pointLight)
}

func setupEvents(app *ReplicubeApp) {
	// when the window is resized
	onResize := func(_ string, _ any) {
		width, height := app.G3NApp.GetSize()
		app.G3NApp.Gls().Viewport(0, 0, int32(width), int32(height))
		app.Elements["cam"].(*camera.Camera).SetAspect(float32(width) / float32(height))
	}
	app.G3NApp.Subscribe(window.OnWindowSize, onResize)
	onResize("", nil)

	// when the mouse is down
	// app.G3NApp.Subscribe(window.OnMouseDown, func(_ string, ev any) {
	// 	mev := ev.(*window.MouseEvent)

	// 	width, height := app.G3NApp.GetSize()
	// 	nx := (2*float32(mev.Xpos)/float32(width) - 1)
	// 	ny := (1 - 2*float32(mev.Ypos)/float32(height))

	// 	raycaster := collision.NewRaycaster(&math32.Vector3{}, &math32.Vector3{})
	// 	raycaster.SetFromCamera(app.Elements["cam"].(*camera.Camera), nx, ny)

	// 	inters := raycaster.IntersectObject(app.Elements["cubeMesh"], true)
	// 	if len(inters) > 0 {
	// 		app.Materials["cubeMaterial"].(*material.Standard).SetColor(math32.NewColor("Red"))
	// 	}
	// })
}

func setupLuaState(app *ReplicubeApp) {
	lua.OpenLibraries(app.LuaState)
	m := map[string]*math32.Color {
		"red": 		{R:1, G:0, B:0},
		"blue": 	{R:0, G:0, B:1},
		"green": 	{R:0, G:1, B:0},
		"yellow": 	{R:1, G:1, B:0},
		"black": 	{R:0, G:0, B:0},
		"white": 	{R:1, G:1, B:1},
	}
	for k, v := range m {
		app.LuaState.PushUserData(v)
		app.LuaState.SetGlobal(k)
	}
}

// the lua fetching functions

func fetchReplicubeLua(app *ReplicubeApp, filename string) {
	// check for errors in the file
	if err := lua.DoFile(app.LuaState, filename); err != nil {
		return
	}
	defer app.LuaState.SetTop(0)

	// check for 1 return value
	if app.LuaState.Top() != 1 {
		return
	}

	// everything ok, try to get the userdata
	col, ok := app.LuaState.ToUserData(1).(*math32.Color)
	if !ok {
		return
	}

	// try to get my material of cube
	mat, ok := app.Materials["cubeMaterial"].(*material.Standard)
	if !ok {
		return
	}
	mat.SetColor(col)
}

func fsLuaWatcherThread(watcher *fsnotify.Watcher, app *ReplicubeApp) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op & fsnotify.Write != fsnotify.Write {
				return
			}
			fetchReplicubeLua(app, event.Name)

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("Error: %s\n", err)
		}
	}
}

func startLuaFileListener(app *ReplicubeApp, filename string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return watcher, err
	}
	go fsLuaWatcherThread(watcher, app)

	err = watcher.Add(filename)
	if err != nil {
		watcher.Close()
		return watcher, err
	}
	fetchReplicubeLua(app, filename)
	return watcher, err
}

// the main functions

func renderStepped(app *ReplicubeApp, dt time.Duration) {
	app.G3NApp.Gls().Clear(gls.DEPTH_BUFFER_BIT | gls.STENCIL_BUFFER_BIT | gls.COLOR_BUFFER_BIT)
	defer app.Renderer.Render(app.Scene, app.Elements["cam"].(*camera.Camera))

	// make the cube rotate
	cube, ok := app.Elements["cubeMesh"].(*graphic.Mesh)
	if !ok {
		log.Fatal("cant cast to cube")
	}
	cube.RotateY(0.001)
}

func giveAppCallback(app *ReplicubeApp, f func(*ReplicubeApp, time.Duration)) func(*renderer.Renderer, time.Duration) {
	return func(r *renderer.Renderer, d time.Duration) {
		app.Renderer = r
		f(app, d)
	}
}

func main() {
	app := &ReplicubeApp{
		G3NApp: app.App(),
		Scene: core.NewNode(),
		Elements: map[string]core.INode{},
		Materials: map[string]material.IMaterial{},
		LuaState: lua.NewState(),
	}

	setupInstances(app)
	setupEvents(app)
	setupLuaState(app)
	watcher, err := startLuaFileListener(app, "replicube.lua")
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	app.G3NApp.Run(giveAppCallback(app, renderStepped))
}
