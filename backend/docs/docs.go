package docs

import "github.com/swaggo/swag"

const docTemplate = `{
  "swagger": "2.0",
  "info": {
    "title": "Expense Tracker API",
    "version": "1.0",
    "description": "Backend API for offline-first shared expense tracking."
  },
  "basePath": "/",
  "schemes": ["http","https"],
  "paths": {
    "/healthz": {
      "get": {
        "summary": "Health check",
        "responses": {
          "200": {
            "description": "OK"
          }
        }
      }
    },
    "/api/v1/sync": {
      "post": {
        "summary": "Sync local state to server",
        "responses": {
          "200": {
            "description": "Sync success"
          }
        }
      }
    }
  }
}`

var SwaggerInfo = &swag.Spec{
	Version:          "1.0",
	Host:             "",
	BasePath:         "/",
	Schemes:          []string{"http", "https"},
	Title:            "Expense Tracker API",
	Description:      "Backend API for offline-first shared expense tracking.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
