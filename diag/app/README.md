# Diagnostic Web App

This is a basic diagnostic web UI. It's written as a small React application, bootstrapped with create-react-app, which is then embedded in its compiled form with Go `embed` into the binary.

## Testing changes

💡 Use `Launch web sample` launch configuration for connecting with backend.

`npm start` to start the development server, and then run a go-workflows sample that hosts the web UI. The `proxy` configuration in `package.json` should allow you to access the web ui from the auto-reloading development server, and serve the API via your Go binary.

## Checking in changes

`npm run build` to update the compiled application in `./build`. The build output has to be committed to the repository, it is embedded by the Go API.
