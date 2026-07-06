package main

import (
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/http/routes"
	subsysteroutes "github.com/routerarchitects/ow-common-mods/fiber/system-routes"
)

func main() {
	publicApp := fiber.New()
	publicDeps := routes.PublicDeps{
		AuthHandler: func(c fiber.Ctx) error { return c.Next() },
		Subsystem:   subsysteroutes.Config{},
	}
	routes.RegisterPublic(publicApp, publicDeps)

	fmt.Println("Registered routes:")
	for _, route := range publicApp.GetRoutes() {
		fmt.Printf("Method: %s, Path: %s\n", route.Method, route.Path)
	}
}
