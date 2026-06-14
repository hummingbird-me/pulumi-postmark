package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mrz1836/postmark"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Template is a Postmark transactional template or layout. Templates live inside
// a single server, so their CRUD uses that server's token (see ServerToken /
// ServerID). The Postmark create/edit calls return only lightweight info, so the
// provider re-reads each template after writing to capture its bodies.
type Template struct{}

var (
	_ infer.CustomResource[TemplateArgs, TemplateState] = (*Template)(nil)
	_ infer.CustomUpdate[TemplateArgs, TemplateState]   = (*Template)(nil)
	_ infer.CustomRead[TemplateArgs, TemplateState]     = (*Template)(nil)
	_ infer.CustomDelete[TemplateState]                 = (*Template)(nil)
	_ infer.CustomCheck[TemplateArgs]                   = (*Template)(nil)
)

func (*Template) Annotate(a infer.Annotator) { a.SetToken("index", "Template") }

type TemplateArgs struct {
	Name           string  `pulumi:"name"`
	Alias          *string `pulumi:"alias,optional"`
	Subject        *string `pulumi:"subject,optional"`
	HTMLBody       *string `pulumi:"htmlBody,optional"`
	TextBody       *string `pulumi:"textBody,optional"`
	TemplateType   *string `pulumi:"templateType,optional" provider:"replaceOnChanges"`
	LayoutTemplate *string `pulumi:"layoutTemplate,optional"`

	// Server token resolution (precedence A → B → C; see resolveServerToken).
	ServerToken *string `pulumi:"serverToken,optional" provider:"secret"`
	ServerID    *int    `pulumi:"serverId,optional" provider:"replaceOnChanges"`
}

type TemplateState struct {
	TemplateArgs
	TemplateID         int  `pulumi:"templateId"`
	Active             bool `pulumi:"active"`
	AssociatedServerID int  `pulumi:"associatedServerId"`
}

func (a *TemplateArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Name, "Name of the template.")
	an.Describe(&a.Alias, "Optional unique alias (within the server). Must start with a letter; may contain "+
		"letters, digits, '.', '-' and '_'.")
	an.Describe(&a.Subject, "Subject line. Required for Standard templates; not allowed for Layouts.")
	an.Describe(&a.HTMLBody, "HTML body. Required unless `textBody` is set.")
	an.Describe(&a.TextBody, "Plain-text body. Required unless `htmlBody` is set.")
	an.Describe(&a.TemplateType, "`Standard` (default) or `Layout`. Immutable — changing it replaces the template.")
	an.Describe(&a.LayoutTemplate, "Alias of a Layout to wrap this Standard template (set to \"\" to disassociate).")
	an.Describe(&a.ServerToken, "Server API token of the server this template belongs to. Wire from "+
		"`server.apiTokens[0]`. Falls back to `serverId` lookup, then the provider `serverToken` config.")
	an.Describe(&a.ServerID, "ID of the server this template belongs to; the provider looks up its token via the "+
		"account token. Immutable — changing it replaces the template.")
}

// Check validates template inputs before create/update.
func (*Template) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[TemplateArgs], error) {
	args, failures, err := infer.DefaultCheck[TemplateArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[TemplateArgs]{Inputs: args, Failures: failures}, err
	}

	templateType := deref(args.TemplateType, "Standard")
	hasBody := deref(args.HTMLBody, "") != "" || deref(args.TextBody, "") != ""

	switch templateType {
	case "Layout":
		if deref(args.Subject, "") != "" {
			failures = append(failures, p.CheckFailure{Property: "subject", Reason: "Layout templates cannot have a subject."})
		}
		if !hasBody {
			failures = append(failures, p.CheckFailure{Property: "htmlBody", Reason: "A Layout requires an htmlBody or textBody."})
		}
	case "Standard":
		if deref(args.Subject, "") == "" {
			failures = append(failures, p.CheckFailure{Property: "subject", Reason: "Standard templates require a subject."})
		}
		if !hasBody {
			failures = append(failures, p.CheckFailure{Property: "htmlBody", Reason: "A Standard template requires an htmlBody or textBody."})
		}
	default:
		failures = append(failures, p.CheckFailure{Property: "templateType", Reason: "templateType must be \"Standard\" or \"Layout\"."})
	}

	return infer.CheckResponse[TemplateArgs]{Inputs: args, Failures: failures}, nil
}

func (*Template) Create(ctx context.Context, req infer.CreateRequest[TemplateArgs]) (infer.CreateResponse[TemplateState], error) {
	if req.DryRun {
		return infer.CreateResponse[TemplateState]{Output: TemplateState{TemplateArgs: req.Inputs}}, nil
	}
	client, err := templateClient(ctx, req.Inputs)
	if err != nil {
		return infer.CreateResponse[TemplateState]{}, err
	}
	info, err := client.CreateTemplate(ctx, templateRequest(req.Inputs))
	if err != nil {
		return infer.CreateResponse[TemplateState]{}, fmt.Errorf("creating template %q: %w", req.Inputs.Name, err)
	}
	// Create returns only TemplateInfo (no bodies); re-read for full state.
	full, err := client.GetTemplate(ctx, strconv.FormatInt(info.TemplateID, 10))
	if err != nil {
		return infer.CreateResponse[TemplateState]{}, fmt.Errorf("reading back template %d after create: %w", info.TemplateID, err)
	}
	state := templateStateFromAPI(full, req.Inputs)
	return infer.CreateResponse[TemplateState]{ID: templateResourceID(full), Output: state}, nil
}

func (*Template) Read(ctx context.Context, req infer.ReadRequest[TemplateArgs, TemplateState]) (infer.ReadResponse[TemplateArgs, TemplateState], error) {
	client, err := templateClient(ctx, req.Inputs)
	if err != nil {
		return infer.ReadResponse[TemplateArgs, TemplateState]{}, err
	}
	_, templateID, err := parseTemplateResourceID(req.ID)
	if err != nil {
		return infer.ReadResponse[TemplateArgs, TemplateState]{}, err
	}
	full, err := client.GetTemplate(ctx, templateID)
	if err != nil {
		if isNotFound(err) {
			return infer.ReadResponse[TemplateArgs, TemplateState]{}, nil
		}
		return infer.ReadResponse[TemplateArgs, TemplateState]{}, fmt.Errorf("reading template %s: %w", req.ID, err)
	}
	state := templateStateFromAPI(full, req.Inputs)
	return infer.ReadResponse[TemplateArgs, TemplateState]{
		ID:     templateResourceID(full),
		Inputs: state.TemplateArgs,
		State:  state,
	}, nil
}

func (*Template) Update(ctx context.Context, req infer.UpdateRequest[TemplateArgs, TemplateState]) (infer.UpdateResponse[TemplateState], error) {
	if req.DryRun {
		out := req.State
		out.TemplateArgs = req.Inputs
		return infer.UpdateResponse[TemplateState]{Output: out}, nil
	}
	client, err := templateClient(ctx, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[TemplateState]{}, err
	}
	_, templateID, err := parseTemplateResourceID(req.ID)
	if err != nil {
		return infer.UpdateResponse[TemplateState]{}, err
	}
	if _, err := client.EditTemplate(ctx, templateID, templateRequest(req.Inputs)); err != nil {
		return infer.UpdateResponse[TemplateState]{}, fmt.Errorf("updating template %s: %w", req.ID, err)
	}
	full, err := client.GetTemplate(ctx, templateID)
	if err != nil {
		return infer.UpdateResponse[TemplateState]{}, fmt.Errorf("reading back template %s after update: %w", req.ID, err)
	}
	return infer.UpdateResponse[TemplateState]{Output: templateStateFromAPI(full, req.Inputs)}, nil
}

func (*Template) Delete(ctx context.Context, req infer.DeleteRequest[TemplateState]) (infer.DeleteResponse, error) {
	client, err := templateClient(ctx, req.State.TemplateArgs)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	_, templateID, err := parseTemplateResourceID(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := client.DeleteTemplate(ctx, templateID); err != nil {
		if isNotFound(err) {
			return infer.DeleteResponse{}, nil
		}
		return infer.DeleteResponse{}, fmt.Errorf("deleting template %s: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

func templateRequest(a TemplateArgs) postmark.Template {
	return postmark.Template{
		Name:           a.Name,
		Subject:        deref(a.Subject, ""),
		HTMLBody:       deref(a.HTMLBody, ""),
		TextBody:       deref(a.TextBody, ""),
		Alias:          deref(a.Alias, ""),
		TemplateType:   deref(a.TemplateType, ""),
		LayoutTemplate: deref(a.LayoutTemplate, ""),
	}
}

func templateStateFromAPI(t postmark.Template, args TemplateArgs) TemplateState {
	echoed := TemplateArgs{
		Name:           t.Name,
		Subject:        ptr(t.Subject),
		HTMLBody:       ptr(t.HTMLBody),
		TextBody:       ptr(t.TextBody),
		TemplateType:   ptr(t.TemplateType),
		LayoutTemplate: ptr(t.LayoutTemplate),
		// Token inputs are not returned by the API; carry them from the request.
		ServerToken: args.ServerToken,
		ServerID:    args.ServerID,
	}
	if t.Alias != "" {
		echoed.Alias = ptr(t.Alias)
	} else {
		echoed.Alias = args.Alias
	}
	return TemplateState{
		TemplateArgs:       echoed,
		TemplateID:         int(t.TemplateID),
		Active:             t.Active,
		AssociatedServerID: int(t.AssociatedServerID),
	}
}

// templateResourceID namespaces a template by its server: "{serverId}/{templateId}".
func templateResourceID(t postmark.Template) string {
	return fmt.Sprintf("%d/%d", t.AssociatedServerID, t.TemplateID)
}

func parseTemplateResourceID(id string) (serverID int, templateID string, err error) {
	parts := strings.SplitN(strings.TrimSpace(id), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, "", fmt.Errorf("invalid template id %q: expected \"{serverId}/{templateId}\"", id)
	}
	serverID, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", fmt.Errorf("invalid template id %q: server segment is not numeric: %w", id, err)
	}
	return serverID, parts[1], nil
}
