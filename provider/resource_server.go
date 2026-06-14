package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mrz1836/postmark"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Server is a Postmark server: a logical sending environment that owns its own
// Server API token(s). It is the resource through which inbound email is wired
// up (inboundHookUrl/inboundDomain) and from which Templates and apps obtain a
// server token (the secret `apiTokens` output).
type Server struct{}

// Compile-time assertions that *Server implements the lifecycle we rely on.
var (
	_ infer.CustomResource[ServerArgs, ServerState] = (*Server)(nil)
	_ infer.CustomUpdate[ServerArgs, ServerState]   = (*Server)(nil)
	_ infer.CustomRead[ServerArgs, ServerState]     = (*Server)(nil)
	_ infer.CustomDelete[ServerState]               = (*Server)(nil)
)

type ServerArgs struct {
	Name                       string  `pulumi:"name"`
	Color                      *string `pulumi:"color,optional"`
	DeliveryType               *string `pulumi:"deliveryType,optional" provider:"replaceOnChanges"`
	SMTPAPIActivated           *bool   `pulumi:"smtpApiActivated,optional"`
	RawEmailEnabled            *bool   `pulumi:"rawEmailEnabled,optional"`
	InboundHookURL             *string `pulumi:"inboundHookUrl,optional"`
	InboundDomain              *string `pulumi:"inboundDomain,optional"`
	InboundSpamThreshold       *int    `pulumi:"inboundSpamThreshold,optional"`
	PostFirstOpenOnly          *bool   `pulumi:"postFirstOpenOnly,optional"`
	TrackOpens                 *bool   `pulumi:"trackOpens,optional"`
	TrackLinks                 *string `pulumi:"trackLinks,optional"`
	IncludeBounceContentInHook *bool   `pulumi:"includeBounceContentInHook,optional"`
	EnableSMTPAPIErrorHooks    *bool   `pulumi:"enableSmtpApiErrorHooks,optional"`
}

type ServerState struct {
	ServerArgs
	ServerID       int      `pulumi:"serverId"`
	APITokens      []string `pulumi:"apiTokens" provider:"secret"`
	ServerLink     string   `pulumi:"serverLink"`
	InboundAddress string   `pulumi:"inboundAddress"`
	InboundHash    string   `pulumi:"inboundHash"`
}

func (a *ServerArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Name, "Friendly name of the server.")
	an.Describe(&a.Color, "Color of the server in the Postmark UI server list. One of: "+
		"Purple, Blue, Turquoise, Green, Red, Yellow, Grey, Orange. Defaults to Blue.")
	an.Describe(&a.DeliveryType, "Environment type: `Live` or `Sandbox`. "+
		"Immutable after creation — changing it replaces the server. Defaults to `Live`.")
	an.Describe(&a.InboundHookURL, "URL Postmark POSTs to for every inbound email event.")
	an.Describe(&a.InboundDomain, "Inbound domain for MX setup. Point an MX record for this "+
		"domain at `inbound.postmarkapp.com` to receive mail.")
	an.Describe(&a.InboundSpamThreshold, "Maximum spam score before an inbound message is blocked.")
	an.Describe(&a.TrackOpens, "Enable open tracking for messages by default.")
	an.Describe(&a.TrackLinks, "Link tracking mode: None, HtmlAndText, HtmlOnly or TextOnly. Defaults to None.")
}

func (*Server) Create(ctx context.Context, req infer.CreateRequest[ServerArgs]) (infer.CreateResponse[ServerState], error) {
	if req.DryRun {
		return infer.CreateResponse[ServerState]{Output: ServerState{ServerArgs: req.Inputs}}, nil
	}

	client, err := accountClient(ctx)
	if err != nil {
		return infer.CreateResponse[ServerState]{}, err
	}

	srv, err := client.CreateServer(ctx, serverCreateRequest(req.Inputs))
	if err != nil {
		return infer.CreateResponse[ServerState]{}, fmt.Errorf("creating server %q: %w", req.Inputs.Name, err)
	}
	return infer.CreateResponse[ServerState]{
		ID:     strconv.FormatInt(srv.ID, 10),
		Output: serverStateFromAPI(srv),
	}, nil
}

func (*Server) Read(ctx context.Context, req infer.ReadRequest[ServerArgs, ServerState]) (infer.ReadResponse[ServerArgs, ServerState], error) {
	client, err := accountClient(ctx)
	if err != nil {
		return infer.ReadResponse[ServerArgs, ServerState]{}, err
	}
	id, err := idToInt64(req.ID)
	if err != nil {
		return infer.ReadResponse[ServerArgs, ServerState]{}, err
	}
	srv, err := client.GetServer(ctx, id)
	if err != nil {
		if isNotFound(err) {
			// Empty ID signals to the engine that the resource is gone.
			return infer.ReadResponse[ServerArgs, ServerState]{}, nil
		}
		return infer.ReadResponse[ServerArgs, ServerState]{}, fmt.Errorf("reading server %s: %w", req.ID, err)
	}
	state := serverStateFromAPI(srv)
	return infer.ReadResponse[ServerArgs, ServerState]{
		ID:     req.ID,
		Inputs: state.ServerArgs,
		State:  state,
	}, nil
}

func (*Server) Update(ctx context.Context, req infer.UpdateRequest[ServerArgs, ServerState]) (infer.UpdateResponse[ServerState], error) {
	if req.DryRun {
		// Preserve computed outputs; reflect new inputs.
		out := req.State
		out.ServerArgs = req.Inputs
		return infer.UpdateResponse[ServerState]{Output: out}, nil
	}

	client, err := accountClient(ctx)
	if err != nil {
		return infer.UpdateResponse[ServerState]{}, err
	}
	id, err := idToInt64(req.ID)
	if err != nil {
		return infer.UpdateResponse[ServerState]{}, err
	}
	srv, err := client.EditServer(ctx, id, serverEditRequest(req.Inputs))
	if err != nil {
		return infer.UpdateResponse[ServerState]{}, fmt.Errorf("updating server %s: %w", req.ID, err)
	}
	return infer.UpdateResponse[ServerState]{Output: serverStateFromAPI(srv)}, nil
}

func (*Server) Delete(ctx context.Context, req infer.DeleteRequest[ServerState]) (infer.DeleteResponse, error) {
	client, err := accountClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	id, err := idToInt64(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := client.DeleteServer(ctx, id); err != nil {
		if isNotFound(err) {
			return infer.DeleteResponse{}, nil
		}
		// Server deletion is not enabled for all Postmark accounts and may fail
		// with a 422 ("contact support"). Surface that clearly.
		return infer.DeleteResponse{}, fmt.Errorf("deleting server %s (note: server deletion may be "+
			"disabled on your Postmark account — contact Postmark support): %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

// --- mapping helpers ---------------------------------------------------------

func serverCreateRequest(a ServerArgs) postmark.ServerCreateRequest {
	return postmark.ServerCreateRequest{
		Name:                       a.Name,
		Color:                      orDefault(deref(a.Color, ""), "Blue"),
		SMTPAPIActivated:           deref(a.SMTPAPIActivated, false),
		RawEmailEnabled:            deref(a.RawEmailEnabled, false),
		DeliveryType:               orDefault(deref(a.DeliveryType, ""), "Live"),
		InboundHookURL:             deref(a.InboundHookURL, ""),
		PostFirstOpenOnly:          deref(a.PostFirstOpenOnly, false),
		InboundDomain:              deref(a.InboundDomain, ""),
		InboundSpamThreshold:       int64(deref(a.InboundSpamThreshold, 0)),
		TrackOpens:                 deref(a.TrackOpens, false),
		TrackLinks:                 orDefault(deref(a.TrackLinks, ""), "None"),
		IncludeBounceContentInHook: deref(a.IncludeBounceContentInHook, false),
		EnableSMTPAPIErrorHooks:    deref(a.EnableSMTPAPIErrorHooks, false),
	}
}

func serverEditRequest(a ServerArgs) postmark.ServerEditRequest {
	// DeliveryType is intentionally omitted: it is immutable (replaceOnChanges).
	return postmark.ServerEditRequest{
		Name:                       a.Name,
		Color:                      orDefault(deref(a.Color, ""), "Blue"),
		SMTPAPIActivated:           deref(a.SMTPAPIActivated, false),
		RawEmailEnabled:            deref(a.RawEmailEnabled, false),
		InboundHookURL:             deref(a.InboundHookURL, ""),
		PostFirstOpenOnly:          deref(a.PostFirstOpenOnly, false),
		InboundDomain:              deref(a.InboundDomain, ""),
		InboundSpamThreshold:       int64(deref(a.InboundSpamThreshold, 0)),
		TrackOpens:                 deref(a.TrackOpens, false),
		TrackLinks:                 orDefault(deref(a.TrackLinks, ""), "None"),
		IncludeBounceContentInHook: deref(a.IncludeBounceContentInHook, false),
		EnableSMTPAPIErrorHooks:    deref(a.EnableSMTPAPIErrorHooks, false),
	}
}

func serverStateFromAPI(s postmark.Server) ServerState {
	return ServerState{
		ServerArgs: ServerArgs{
			Name:                       s.Name,
			Color:                      ptr(s.Color),
			DeliveryType:               ptr(s.DeliveryType),
			SMTPAPIActivated:           ptr(s.SMTPAPIActivated),
			RawEmailEnabled:            ptr(s.RawEmailEnabled),
			InboundHookURL:             ptr(s.InboundHookURL),
			InboundDomain:              ptr(s.InboundDomain),
			InboundSpamThreshold:       ptr(int(s.InboundSpamThreshold)),
			PostFirstOpenOnly:          ptr(s.PostFirstOpenOnly),
			TrackOpens:                 ptr(s.TrackOpens),
			TrackLinks:                 ptr(s.TrackLinks),
			IncludeBounceContentInHook: ptr(s.IncludeBounceContentInHook),
			EnableSMTPAPIErrorHooks:    ptr(s.EnableSMTPAPIErrorHooks),
		},
		ServerID:       int(s.ID),
		APITokens:      s.APITokens,
		ServerLink:     s.ServerLink,
		InboundAddress: s.InboundAddress,
		InboundHash:    s.InboundHash,
	}
}
