# Users Service

The User management service.  Provides endpoints for the signup workflow and user authentication.

## Model

There are 3 types this service understands:

### Users

A users is an individual using the service.  The effective primary key is email address.

User authenticate against the service using a cookie.  Users are given this cookie by following a link in their email (hence they signup for the service by giving us their email address).  Users can also obtain a cookie by associating their account with a Google or Github OAuth authenication provider.

### Logins

A login is a relationship between a user and an authentication provider.  They are created when you "Attach" an OAuth provider to your account.  They contain the session data and an ID we need to access their remote account.

If a user signs up via an OAuth provider and an user record exists with the same email address, they will automatically be attached.

### Organisation

An organisation is the unit of data-ownership within the service.  An organisation has its own authentication token, used for probes to send data to the organisation.

There is a n-to-m relationship between organisations and users.  Memberships is the join-table between them. This is what is created when you invite a user to your organization.

## Development

- Build: You should build all components via the toplevel Makefile.
- Run: See instructions in the toplevel README about running a local cluster.
- Test:

```sh
$ env GO15VENDOREXPERIMENT=1 go test
```

## Security

### User session secret generation

The user management service signs the session cookies with HMAC. In order to do
that, it needs a key which is provided on the command line with the `--session-key`
argument.

In order to generate a key from the command line you can do:

```sh
$ cat /dev/random|xxd -c 2000 -ps|head -c64; echo`
```
