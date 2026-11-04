// Package notify affiche les notifications de bureau (org.freedesktop.Notifications).
// Toutes les méthodes sont sans effet sur un *Notifier nil : sans bus de session
// (SSH, conteneur), l'application fonctionne simplement sans notifications.
package notify

import (
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	dest  = "org.freedesktop.Notifications"
	path  = dbus.ObjectPath("/org/freedesktop/Notifications")
	iface = "org.freedesktop.Notifications"
)

// Notifier envoie des notifications qui se remplacent les unes les autres.
type Notifier struct {
	conn *dbus.Conn
	mu   sync.Mutex
	last uint32
}

// New se connecte au bus de session.
func New() (*Notifier, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	return &Notifier{conn: conn}, nil
}

// Close ferme la connexion.
func (n *Notifier) Close() {
	if n != nil && n.conn != nil {
		n.conn.Close()
	}
}

// Hints construit les indications envoyées avec la notification : discrète (urgence basse,
// catégorie musique) et remplaçable sur les serveurs qui gèrent la notion de synchronisation.
func Hints() map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"urgency":                         dbus.MakeVariant(byte(0)),
		"category":                        dbus.MakeVariant("x-gnome.music"),
		"transient":                       dbus.MakeVariant(true),
		"x-canonical-private-synchronous": dbus.MakeVariant("yt-tui"),
		"desktop-entry":                   dbus.MakeVariant("yt-tui"),
	}
}

// Show affiche (ou remplace) la notification. icon est un chemin de fichier ou un nom
// d'icône. L'appel est asynchrone : il ne bloque jamais l'interface.
func (n *Notifier) Show(summary, body, icon string) {
	if n == nil || n.conn == nil {
		return
	}
	n.mu.Lock()
	replaces := n.last
	n.mu.Unlock()
	if icon == "" {
		icon = "audio-x-generic"
	}
	obj := n.conn.Object(dest, path)
	call := obj.Go(iface+".Notify", 0, make(chan *dbus.Call, 1),
		"yt-tui", replaces, icon, summary, body, []string{}, Hints(), int32(4000))
	go func() {
		<-call.Done
		var id uint32
		if call.Err == nil && call.Store(&id) == nil {
			n.mu.Lock()
			n.last = id
			n.mu.Unlock()
		}
	}()
}
