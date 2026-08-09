package api

import (
	"context"
	"fmt"
	"time"

	beeperdesktopapi "github.com/beeper/desktop-api-go/v5"
)

// SearchContacts searches contacts across all accounts by name, handle, or
// identifier. Accounts that fail to search are skipped so one broken bridge
// does not hide the others' results.
func (c *Client) SearchContacts(ctx context.Context, query string) ([]Contact, error) {
	accounts, err := c.sdk.Accounts.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("api: list accounts: %w", err)
	}
	var out []Contact
	for _, a := range *accounts {
		res, err := c.sdk.Accounts.Contacts.Search(ctx, a.AccountID, beeperdesktopapi.AccountContactSearchParams{Query: query})
		if err != nil {
			continue
		}
		for _, u := range res.Items {
			if u.IsSelf {
				continue
			}
			out = append(out, Contact{
				AccountID:   a.AccountID,
				UserID:      u.ID,
				FullName:    u.FullName,
				Username:    u.Username,
				PhoneNumber: u.PhoneNumber,
				Email:       u.Email,
			})
		}
	}
	return out, nil
}

// contactListCap bounds how many contacts one account contributes, so a huge
// address book cannot stall a tool call.
const contactListCap = 2000

// contactsTTL is how long a synced contact set stays fresh. Contacts change
// rarely, so callers query the cached set and re-sync only after this.
const contactsTTL = 15 * time.Minute

// ListContacts returns the contact set across all accounts, syncing from the
// network only when the cached set is stale. Accounts whose bridge does not
// support listing are skipped.
func (c *Client) ListContacts(ctx context.Context) ([]Contact, error) {
	c.contactsMu.Lock()
	defer c.contactsMu.Unlock()
	if !c.contactsAt.IsZero() && time.Since(c.contactsAt) < contactsTTL {
		return c.contacts, nil
	}
	contacts, err := c.syncContacts(ctx)
	if err != nil {
		return nil, err
	}
	c.contacts, c.contactsAt = contacts, time.Now()
	return contacts, nil
}

// syncContacts fetches every account's contacts from the network.
func (c *Client) syncContacts(ctx context.Context) ([]Contact, error) {
	accounts, err := c.sdk.Accounts.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("api: list accounts: %w", err)
	}
	var out []Contact
	for _, a := range *accounts {
		iter := c.sdk.Accounts.Contacts.ListAutoPaging(ctx, a.AccountID, beeperdesktopapi.AccountContactListParams{})
		n := 0
		for iter.Next() && n < contactListCap {
			u := iter.Current()
			if u.IsSelf {
				continue
			}
			out = append(out, Contact{
				AccountID:   a.AccountID,
				UserID:      u.ID,
				FullName:    u.FullName,
				Username:    u.Username,
				PhoneNumber: u.PhoneNumber,
				Email:       u.Email,
			})
			n++
		}
	}
	return out, nil
}
