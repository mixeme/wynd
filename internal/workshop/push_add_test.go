package workshop

import (
	"strings"
	"testing"
)

func TestPlanPushAddChecksExistingProjectWithoutWrites(t *testing.T) {
	l, err := NewLayout("/srv/wynd-existing")
	if err != nil {
		t.Fatal(err)
	}
	const state = "@image wynd:9.9.9\n@url https://family.example.ru\n@running wynd\n@container wynd\n"
	for _, test := range []struct {
		name, domain, extraState, env, services, caddyID, want string
		filesExist                                             bool
	}{
		{
			name: "existing installation", domain: "https://Push.Family.Example.RU/",
			env: "OPERATOR=value\n", services: "wynd\ncaddy\n", caddyID: "local-caddy",
		},
		{
			name: "wrong DNS", domain: "elsewhere.example.ru", services: "wynd\ncaddy\n",
			caddyID: "local-caddy", want: "указывает",
		},
		{
			name: "already installed", domain: "push.family.example.ru", extraState: "@push\n",
			want: "уже настроен",
		},
		{
			name: "leftover files", domain: "push.family.example.ru", filesExist: true,
			want: "отличаются",
		},
		{
			name: "operator compose override", domain: "push.family.example.ru", env: "COMPOSE_FILE=other.yaml\n",
			want: "отличаются",
		},
		{
			name: "external Caddy", domain: "push.family.example.ru", services: "wynd\n",
			want: "Caddy Мастерской не найден",
		},
		{
			name: "stopped Caddy", domain: "push.family.example.ru", services: "wynd\ncaddy\n",
			want: "Caddy Мастерской не отвечает",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			sess := dialTestHost(t, func(command string, input []byte) (string, int) {
				switch {
				case len(input) != 0:
					t.Errorf("read-only plan sent input: %s", command)
				case command == l.StateScript():
					return state + test.extraState, 0
				case strings.HasSuffix(command, "echo WYND_PUSH_ROOT"):
					return "WYND_PUSH_ROOT\n", 0
				case strings.Contains(command, "for f in push.json push.yaml ntfy.yml"):
					if test.filesExist {
						return "", 3
					}
					return "", 0
				case strings.HasPrefix(command, "if [ -L "+l.Root()+"/.env ];"):
					return test.env, 0
				case strings.HasPrefix(command, "if [ -L "+l.Caddyfile()+" ];"):
					return CaddyFile(Spec{Domain: "family.example.ru"}), 0
				case command == l.ComposeCommand()+"-f compose.yaml config --services 2>/dev/null":
					return test.services, 0
				case command == l.ComposeCommand()+"-f compose.yaml ps --quiet caddy 2>/dev/null":
					return test.caddyID, 0
				default:
					t.Errorf("unexpected command during planning: %s", command)
				}
				return "", 1
			})
			w := &Wizard{
				session: sess, report: &Report{}, Root: l.Root(),
				Resolver: fakeResolver{
					"push.family.example.ru": {"127.0.0.1"},
					"elsewhere.example.ru":   {"192.0.2.5"},
				},
			}
			plan := w.PlanPushAdd(t.Context(), test.domain)
			if test.want == "" {
				if !plan.OK || plan.Domain != "push.family.example.ru" {
					t.Fatalf("plan: %+v", plan)
				}
			} else if plan.OK || !strings.Contains(plan.Message, test.want) {
				t.Fatalf("expected %q, got %+v", test.want, plan)
			}
		})
	}
}
