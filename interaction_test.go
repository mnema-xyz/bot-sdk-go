package mnema

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func decodeInteraction(t *testing.T, raw string, r *REST) *Interaction {
	t.Helper()
	var i Interaction
	if err := json.Unmarshal([]byte(raw), &i); err != nil {
		t.Fatal(err)
	}
	i.rest = r
	return &i
}

const commandJSON = `{"id":"i1","token":"tok","type":2,"channel_id":"c","application_id":"b",
 "data":{"id":"cmd","name":"roll","options":[{"name":"sides","type":4,"value":20},{"name":"loud","type":5,"value":true},{"name":"text","type":3,"value":"hi"}]},
 "member":{"user":{"id":"u","username":"max","display_name":"Max"}}}`

func TestCommandInteractionsExposeTheirOptionsTyped(t *testing.T) {
	i := decodeInteraction(t, commandJSON, nil)
	if !i.IsCommand() || i.IsComponent() || i.CommandName() != "roll" || i.User.DisplayName != "Max" {
		t.Fatalf("interaction = %+v", i)
	}
	if i.Option("sides").Int() != 20 || !i.Option("loud").Bool() || i.Option("text").String() != "hi" {
		t.Fatal("typed option getters")
	}
	if i.Option("missing").String() != "" || i.Option("missing").Int() != 0 || i.Option("text").Int() != 0 {
		t.Fatal("a missing or mistyped option is not the zero value")
	}
}

func TestComponentAndModalInteractionsExposeCustomIDsValuesAndFields(t *testing.T) {
	sel := decodeInteraction(t, `{"member":{"user":{"id":"u0","username":"max"}},"type":3,"message":{"id":"m"},"data":{"custom_id":"env","component_type":3,"values":["prod"]}}`, nil)
	if !sel.IsSelect() || sel.IsButton() || sel.CustomID() != "env" || sel.Values()[0] != "prod" || sel.CommandName() != "" {
		t.Fatalf("select = %+v", sel)
	}
	users := decodeInteraction(t, `{"member":{"user":{"id":"u0","username":"max"}},"type":3,"data":{"custom_id":"who","component_type":5,"values":["u"],"resolved":{"users":{"u":{"id":"u","username":"max"}}}}}`, nil)
	if !users.IsSelect() || users.Data.Resolved.Users["u"].Username != "max" {
		t.Fatalf("user select = %+v", users)
	}
	modal := decodeInteraction(t, `{"member":{"user":{"id":"u0","username":"max"}},"type":5,"data":{"custom_id":"feedback","components":[{"type":1,"components":[{"type":4,"custom_id":"subject","value":"Login"}]}]}}`, nil)
	if !modal.IsModalSubmit() || modal.Field("subject") != "Login" || modal.Field("body") != "" {
		t.Fatalf("modal = %+v", modal)
	}
}

func TestResponsesUseTheDocumentedCallbacksAndRoutes(t *testing.T) {
	r, reqs := restServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		if r.Method == http.MethodPatch || r.URL.Path == "/api/bot/v1/interactions/i1/tok/followups" {
			writeJSON(w, 200, Message{ID: "reply"})
			return
		}
		w.WriteHeader(204)
	})
	ctx := context.Background()
	cmd := decodeInteraction(t, commandJSON, r)
	if err := cmd.DeferReply(ctx, true); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Reply(ctx, Response{Content: "x"}); !errors.Is(err, ErrAlreadyAnswered) {
		t.Fatalf("second answer = %v", err)
	}
	if m, err := cmd.EditReply(ctx, Edit{Content: Ptr("done")}); err != nil || m.ID != "reply" {
		t.Fatalf("EditReply = %+v, %v", m, err)
	}
	if _, err := cmd.FollowUp(ctx, Response{Content: "more", Ephemeral: true}); err != nil {
		t.Fatal(err)
	}
	if err := cmd.DeleteReply(ctx); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Update(ctx, Edit{}); !errors.Is(err, ErrNotFromComponent) {
		t.Fatalf("Update on a command = %v", err)
	}
	click := decodeInteraction(t, `{"member":{"user":{"id":"u0","username":"max"}},"id":"i2","token":"tok","type":3,"message":{"id":"m"},"data":{"custom_id":"b","component_type":2}}`, r)
	if err := click.Update(ctx, Edit{Content: Ptr("clicked"), Components: &[]Component{}}); err != nil {
		t.Fatal(err)
	}
	other := decodeInteraction(t, `{"member":{"user":{"id":"u0","username":"max"}},"id":"i3","token":"tok","type":3,"message":{"id":"m"},"data":{"custom_id":"b","component_type":2}}`, r)
	if err := other.ShowModal(ctx, NewModal("fb", "Feedback", TextInput{CustomID: "subject", Label: "Subject"})); err != nil {
		t.Fatal(err)
	}
	want := []struct{ method, path, body string }{
		{"POST", "/api/bot/v1/interactions/i1/tok/callback", `{"data":{"flags":64},"type":5}`},
		{"PATCH", "/api/bot/v1/interactions/i1/tok/messages/@original", `{"content":"done"}`},
		{"POST", "/api/bot/v1/interactions/i1/tok/followups", `{"content":"more","flags":64}`},
		{"DELETE", "/api/bot/v1/interactions/i1/tok/messages/@original", ``},
		{"POST", "/api/bot/v1/interactions/i2/tok/callback", `{"data":{"content":"clicked","components":[]},"type":7}`},
		{"POST", "/api/bot/v1/interactions/i3/tok/callback", `{"data":{"custom_id":"fb","title":"Feedback","components":[{"type":1,"components":[{"type":4,"custom_id":"subject","style":1,"label":"Subject"}]}]},"type":9}`},
	}
	got := reqs()
	if len(got) != len(want) {
		t.Fatalf("%d requests, want %d", len(got), len(want))
	}
	for k, w := range want {
		if got[k].method != w.method || got[k].path != w.path || string(got[k].body) != w.body {
			t.Errorf("request %d = %s %s %s\nwant %s %s %s", k, got[k].method, got[k].path, got[k].body, w.method, w.path, w.body)
		}
	}
}

func TestReplyAndDeferUpdateSendTheirCallbackTypes(t *testing.T) {
	r, reqs := restServer(t, func(w http.ResponseWriter, _ *http.Request, _ int) { w.WriteHeader(204) })
	ctx := context.Background()
	if err := decodeInteraction(t, commandJSON, r).Reply(ctx, Response{Content: "pong", Ephemeral: true}); err != nil {
		t.Fatal(err)
	}
	click := decodeInteraction(t, `{"member":{"user":{"id":"u0","username":"max"}},"id":"i2","token":"tok","type":3,"data":{"custom_id":"b","component_type":2}}`, r)
	if err := click.DeferUpdate(ctx); err != nil {
		t.Fatal(err)
	}
	got := reqs()
	if string(got[0].body) != `{"data":{"content":"pong","flags":64},"type":4}` || string(got[1].body) != `{"type":6}` {
		t.Fatalf("bodies = %s, %s", got[0].body, got[1].body)
	}
}

func TestAModalSubmitCannotOpenAnotherModal(t *testing.T) {
	i := decodeInteraction(t, `{"member":{"user":{"id":"u0","username":"max"}},"type":5,"data":{"custom_id":"x"}}`, nil)
	if err := i.ShowModal(context.Background(), NewModal("y", "Y", TextInput{CustomID: "a", Label: "A"})); err == nil {
		t.Fatal("a modal submit opened a modal")
	}
}

func TestInteractionPermissions(t *testing.T) {
	// Member with ViewChannel (2) and SendMessages (4) = 6
	i := decodeInteraction(t, `{"member":{"user":{"id":"u","username":"max"},"permissions":"6"},"type":2,"data":{"name":"ping"}}`, nil)
	if i.Member.Permissions != "6" {
		t.Fatalf("permissions = %q, want 6", i.Member.Permissions)
	}
	if !i.HasPermission(PermissionViewChannel) || !i.HasPermission(PermissionSendMessages) {
		t.Fatal("expected ViewChannel and SendMessages to be true")
	}
	if i.HasPermission(PermissionAdministrator) || i.HasPermission(PermissionBanMembers) {
		t.Fatal("expected Admin and Ban to be false")
	}

	// Administrator (1) grants all permissions
	admin := decodeInteraction(t, `{"member":{"user":{"id":"u","username":"admin"},"permissions":"1"},"type":2,"data":{"name":"ping"}}`, nil)
	if !admin.HasPermission(PermissionBanMembers) || !admin.HasPermission(PermissionAdministrator) {
		t.Fatal("admin bit must grant all permissions")
	}

	// Missing permissions (empty/nil)
	empty := decodeInteraction(t, `{"member":{"user":{"id":"u","username":"user"}},"type":2,"data":{"name":"ping"}}`, nil)
	if empty.HasPermission(PermissionViewChannel) {
		t.Fatal("missing permissions must return false")
	}

	var nilMember *InteractionMember
	if nilMember.HasPermission(PermissionViewChannel) {
		t.Fatal("nil member must return false")
	}
}

func TestCommandInputPermissionsMarshal(t *testing.T) {
	tests := []struct {
		input CommandInput
		want  string
	}{
		{
			input: CommandInput{Name: "ban", DefaultMemberPermissions: PermissionBanMembers},
			want:  `{"name":"ban","description":"","options":null,"default_member_permissions":"524288"}`,
		},
		{
			input: CommandInput{Name: "combo", DefaultMemberPermissions: PermissionAdministrator | PermissionViewChannel},
			want:  `{"name":"combo","description":"","options":null,"default_member_permissions":"3"}`,
		},
		{
			input: CommandInput{Name: "str", DefaultMemberPermissions: "6"},
			want:  `{"name":"str","description":"","options":null,"default_member_permissions":"6"}`,
		},
		{
			input: CommandInput{Name: "arr", DefaultMemberPermissions: []string{"ban_members"}},
			want:  `{"name":"arr","description":"","options":null,"default_member_permissions":["ban_members"]}`,
		},
		{
			input: CommandInput{Name: "none"},
			want:  `{"name":"none","description":"","options":null}`,
		},
	}
	for _, tc := range tests {
		b, err := json.Marshal(tc.input)
		if err != nil {
			t.Fatalf("Marshal(%s): %v", tc.input.Name, err)
		}
		if string(b) != tc.want {
			t.Errorf("Marshal(%s) = %s, want %s", tc.input.Name, string(b), tc.want)
		}
	}
}
