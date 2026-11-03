package requester

import (
	"os"
	"testing"
)

func Test_hasChallange(t *testing.T) {
	antiBot, err := os.ReadFile("testdata/vaca_antibot.html")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"cloudflare", "<title>Just a moment...</title>", true},
		{"vaca torrent anti-bot page", string(antiBot), true},
		{"vaca torrent anti-bot raw text", "<h2 id=\"v-title\">Verificando Acesso</h2>", true},
		{"vaca torrent anti-bot without accents", "Protecao Anti-Bot", true},
		{"regular page", "<html><title>Vaqueiro Filmes</title></html>", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasChallange([]byte(tt.body)); got != tt.want {
				t.Errorf("hasChallange() = %v, want %v", got, tt.want)
			}
		})
	}
}
