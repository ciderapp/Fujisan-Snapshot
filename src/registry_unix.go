//go:build !windows

package main

func (c *Cider) RegisterCallbackUrl() error {
	return nil
}
