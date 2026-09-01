// Command keys maintains the master keys that wrap vault data keys.
//
//	keys status      show which master key version each vault's data key uses
//	keys rotate-kek  re-wrap every data key with the current master key (MASTER_KEK)
//
// To rotate the master key: move the old key to MASTER_KEK_PREVIOUS as "1:<old hex key>",
// set MASTER_KEK to a new key and MASTER_KEK_VERSION to 2, restart the API, run
// "keys rotate-kek", then remove the old key from MASTER_KEK_PREVIOUS and restart again.
package main

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/config"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/crypto"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/keys"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/storage"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "keys: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 || (args[0] != "status" && args[0] != "rotate-kek") {
		return fmt.Errorf("usage: keys status | rotate-kek")
	}

	if err := config.Load(); err != nil {
		return err
	}
	keyring, err := crypto.LoadKeyringFromEnv()
	if err != nil {
		return err
	}

	ctx := context.Background()
	pool, err := storage.NewPostgresPool(ctx, config.Database())
	if err != nil {
		return err
	}
	defer pool.Close()

	if args[0] == "rotate-kek" {
		result, err := keys.RotateKEK(ctx, pool, keyring)
		fmt.Printf("Re-wrapped %d vault data key(s) with master key version %d\n", result.Rewrapped, result.Current)
		if err != nil {
			return err
		}
	}

	counts, err := keys.VersionCounts(ctx, pool)
	if err != nil {
		return err
	}
	versions := make([]int, 0, len(counts))
	for v := range counts {
		versions = append(versions, v)
	}
	sort.Ints(versions)

	fmt.Printf("Current master key version: %d (configured: %v)\n", keyring.CurrentVersion(), keyring.Versions())
	if len(versions) == 0 {
		fmt.Println("No vaults yet")
	}
	for _, v := range versions {
		note := ""
		switch {
		case !keyring.Has(v):
			note = "  NOT CONFIGURED: these vaults cannot be decrypted"
		case v != keyring.CurrentVersion():
			note = "  run: keys rotate-kek"
		}
		fmt.Printf("  version %d: %d vault(s)%s\n", v, counts[v], note)
	}

	var retirable []int
	for _, v := range keyring.Versions() {
		if v != keyring.CurrentVersion() && counts[v] == 0 {
			retirable = append(retirable, v)
		}
	}
	if len(retirable) > 0 {
		fmt.Printf("No vault uses version(s) %v any more; they can be removed from MASTER_KEK_PREVIOUS\n", retirable)
	}
	return keys.CheckKeyring(ctx, pool, keyring)
}
