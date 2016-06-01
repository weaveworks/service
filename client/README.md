# client

React app for the Scope service.

## Build

You should build all components via the toplevel Makefile.
But, if you're just working on the client, you can try:

```sh
npm install
npm run build
```

Copy the contents of `build/` to your static webserver's document root directory.

## Run

See instructions in the toplevel README about running a local cluster.
But, to just run the local client:

```sh
npm install
npm start
```

Navigate to **http://localhost:4046/** to view the app.

To move past the landing page, you need to run the `users` service with `-direct-login`:

```sh
cd ../users
go build
./users -port 4047 -email-uri log:// -direct-login -database-uri memory://
```
The `My Scope` will try to load a local Scope running on `http://localhost:4042`.

## Test

```sh
$ npm test
```
