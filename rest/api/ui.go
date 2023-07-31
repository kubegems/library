package api

import (
	"bytes"
	"html/template"
	"net/http"
)

const (
	RedocTemplate = `<!DOCTYPE html>
	<html>
	  <head>
		<title>API Documentation</title>
			<!-- needed for adaptive design -->
			<meta charset="utf-8"/>
			<meta name="viewport" content="width=device-width, initial-scale=1">
			<link href="https://fonts.googleapis.com/css?family=Montserrat:300,400,700|Roboto:300,400,700" rel="stylesheet">
	
		<!--
		ReDoc doesn't change outer page styles
		-->
		<style>
		  body {
			margin: 0;
			padding: 0;
		  }
		</style>
	  </head>
	  <body>
		<redoc spec-url='{{ .SpecURL }}'></redoc>
		<script src="https://cdn.jsdelivr.net/npm/redoc/bundles/redoc.standalone.js"> </script>
	  </body>
	</html>
	`
	SwaggerTemplate = `
	<!DOCTYPE html>
	<html lang="en">
	  <head>
		<meta charset="UTF-8">
			<title>API Documentation</title>
	
		<link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist/swagger-ui.css" >
		<link rel="icon" type="image/png" href="https://unpkg.com/swagger-ui-dist/favicon-32x32.png" sizes="32x32" />
		<link rel="icon" type="image/png" href="https://unpkg.com/swagger-ui-dist/favicon-16x16.png" sizes="16x16" />
		<style>
		  html
		  {
			box-sizing: border-box;
			overflow: -moz-scrollbars-vertical;
			overflow-y: scroll;
		  }
		  *,
		  *:before,
		  *:after
		  {
			box-sizing: inherit;
		  }
		  body
		  {
			margin:0;
			background: #fafafa;
		  }
		</style>
	  </head>
	  <body>
		<div id="swagger-ui"></div>
		<script src="https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js"> </script>
		<script src="https://unpkg.com/swagger-ui-dist/swagger-ui-standalone-preset.js"> </script>
		<script>
		window.onload = function() {
		  // Begin Swagger UI call region
		  const ui = SwaggerUIBundle({
			url: '{{ .SpecURL }}',
			dom_id: '#swagger-ui',
			deepLinking: true,
			presets: [
			  SwaggerUIBundle.presets.apis,
			  SwaggerUIStandalonePreset
			],
			plugins: [
			  SwaggerUIBundle.plugins.DownloadUrl
			],
			layout: "StandaloneLayout",
			oauth2RedirectUrl: '{{ .OAuthCallbackURL }}'
		  })
		  // End Swagger UI call region
	
		  window.ui = ui
		}
	  </script>
	  </body>
	</html>
	`
)

func NewSwaggerUI(specPath string) []byte {
	return render(specPath, SwaggerTemplate)
}

func NewRedocUI(specPath string) []byte {
	return render(specPath, RedocTemplate)
}

func render(specPath string, htmltemplate string) []byte {
	tmpl := template.Must(template.New("ui").Parse(htmltemplate))
	buf := bytes.NewBuffer(nil)
	_ = tmpl.Execute(buf, map[string]string{
		"SpecURL": specPath,
	})
	return buf.Bytes()
}

func renderHTML(w http.ResponseWriter, html []byte) {
	w.Header().Set("Content-Type", "text/html")
	w.Write(html)
}
