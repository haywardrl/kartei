// Package slipbox is the engine behind the room: a folder of markdown cards
// plus one JSON sidecar, exposed as a queryable, mutable model. It knows
// nothing about any user interface, network, or database.
//
// # The model
//
//   - A card is a markdown file the app named: ID--slug.md, where the ID is
//     the creation timestamp. The ID is the identity and lives nowhere else.
//     Files the app did not name are guests: readable, linkable, never
//     rewritten, never filed.
//   - A box is a slip box. There are two by default: "main" for permanent
//     notes and "lit" for literature notes (Luhmann kept a separate
//     bibliographic box). Boxes are declared in Settings.Boxes.
//   - The tree is the filing. A card with a tree entry is in a box: behind a
//     parent card, or a root of a box when the parent is empty. A card with
//     no entry is unfiled and lies on the desk. Parent pointers are the only
//     structure stored; everything else is derived.
//   - An address (1b3, or L2a in the literature box) is derived from the tree
//     at load time and never written anywhere but _index.md.
//   - A drawer is a physical chunk of a box: filled a whole branch at a time by default,
//     or equal chunks of the address sequence under the address-range rule.
//   - Links are [[wikilinks]] in the body, written by the user, resolved by
//     ID, slug, title or address. They cross boxes freely.
//   - The desk holds exactly the unfiled cards; only their positions are
//     stored. Boards pin filed cards by reference. The register is the user's
//     own index of terms.
//
// # Invariants the engine keeps
//
//   - Every write is atomic and inside the vault root (atomic.go).
//   - A note that changed on disk is never overwritten silently (write.go).
//   - Damaged and guest files are never rewritten (note.go, write.go).
//   - The sidecar is backed up once per session and recovered from backup
//     when unreadable; a half-synced folder never causes placements to be
//     stripped (state.go).
//   - _index.md mirrors the tree in plain markdown and can rebuild it if the
//     sidecar is lost (index.go).
//
// # Where things are
//
//   - vault.go: Open, indexing, and every read method.
//   - write.go: creating, updating, deleting and filing cards.
//   - surfaces.go: desk, boards, register, prefs, save and events.
//   - tree.go, drawer.go: address and drawer derivation.
//   - state.go: the sidecar schema, migration, backups.
//   - watch.go: reflecting edits made in other editors.
//
// To add a derived view, compute it in reindex and expose a read method. To
// add stored structure, add it to State, bump stateVersion and extend
// migrate. Never put structure into a note's frontmatter.
package slipbox
