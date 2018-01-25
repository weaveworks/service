
# Config Manager API

## General

* Send JSON request body
* Receive JSON response on success
* On error, receive code and either JSON response containing error message as string or "null"
* Create with `POST /:resource`, get with `GET /:resource/:id`, update with `PUT /:resource/:id`, delete with `DELETE /:resource/:id`

## Resources

### Event Type

An event type represents a kind of event that may be sent. It consists of:

* `name` string: Internal identifier (eg. `flux_deploy`)

* `display_name` string: User-visible identifier (eg. `Service Deployed`)

* `description` string: User-visible descriptive/help text
(eg. `Notifications for when Weave Deploy deploys a service to your cluster`)

* `default_receiver_types` list of string: List of receiver types (eg. `email`, `slack`, `browser`)
that this kind of event should be sent to by default.

### Receiver

A place messages can be sent, and config for what kind of messages it should receive. It consists of:

* `id` string: A UUID identifying this receiver

* `type` string: The receiver type, eg. `email`, `slack`, `browser`

* `instance_id` string: The OrgID/instance ID for the Cloud instance associated with this receiver

* `eventTypes` list of string: The names of the event types that should be sent to this receiver.

* `address_data` varies: A JSON value (object, string, etc) identifying how to send to this particular receiver.
Its schema is dependent on the receiver type. For example, for email it might be just a string (the email address),
whereas for some other type it may be an object containing multiple pieces of information.

### Event

An individual thing that occurred that triggered notifications to possibly go out.
It consists of:

* `type` string: The event type of the event (eg. `flux_deploy`)

* `instance_id` string: The OrgID/instance ID for the Cloud instance associated with this event

* `timestamp` string: An RFC 3339 timestamp string ("YYYY-MM-DD HH:mm:ss.sss") indiciating when the event occurred.

* `messages` map from string to any JSON: Different ways of displaying the message. For example, might have a 'slack' entry
which indicates the slack markup text that should be used.

## Unauthenticated (globally callable)

### List Event Types

`GET /notification/config/eventtypes`

Returns a list of all event types that exist, with info as described under Event Types above.

## Authenticated (user callable)

Authenticated endpoints have an Org ID (aka. instance ID) associated. The authentication is done by authfe.

### List Receivers

`GET /notification/config/receivers`

Return a list of all receivers associated with the authenticated instance.

### Create Receiver

`POST /notification/config/receivers`

Create a new receiver for the authenticated instance.
The request body JSON should not specify receiver ID, instance ID or any event types
(these fields may be empty, or preferably omitted entirely).

The new receiver will have an initial list of event types as per event type defaults.

The response will be status 201 Created and contain a JSON string containing the new receiver id.

Errors:

* 400 Bad Request: The given receiver specified id, instance or event types, or had bad address data.

### Get Receiver

`GET /notification/config/receivers/:id`

Returns the receiver with the given receiver id.

Errors:

* 404 Not Found: No receiver exists with that id, or receiver is not associated with the authenticated instance

### Update Receiver

`PUT /notification/config/receivers/:id`

Update existing receiver with given id to the values given in the request body.

Receiver ID and instance ID must be given in the request body, and must match the id in the url
and the authenticated instance respectively.

Receiver type cannot be modified and must match the original value.

On error, an error message may be returned as a JSON string with further explanation.

Errors:

* 400 Bad Request: Receiver ID, instance ID or receiver type did not match, or event types contained non-existent event type.
* 404 Not Found: No receiver exists with that id, or receiver is not associated with the authenticated instance

### Delete Receiver

`DELETE /notification/config/receivers/:id`

Permanently delete a receiver.

Errors:

* 404 Not Found: No receiver exists with that id, or receiver is not associated with the authenticated instance

### List Receivers For Event

`GET /notification/config/receivers_for_event/:name`

This call is intended for internal use when sending an event.

It returns a list of all receivers associated with the authenticated instance
which have the given event type name in their list of event types.

### Add New Event

`POST /notification/events`

This call is intended for internal use when an event has been sent.

It creates a new event entry in the database for later retreival.

The instance ID must match the authenticated instance.

Errors:

* 400 Bad Request: instance ID did not match

### Get Events

`GET /notification/events`

Query Parameters:

* `limit` int: How many events to return, up to 100. Default 50.

* `offset` int: How many events to skip before the first event to return. Default 0.

Return a list of up to `limit` most recent events to occur, starting from `offset` events ago.

Errors:

* 400 Bad Request: limit or offset not integer or out of range
