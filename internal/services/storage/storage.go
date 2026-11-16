// Package storage holds the data model of hushtongue, shared by the services
// and their stores. A store's dictionary methods act on the dictionary of
// the userID they are given only: an id of another user's word or
// translation is reported as ErrNotFound.
package storage
