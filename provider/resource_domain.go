package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mrz1836/postmark"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// Domain is a Postmark sending domain. Create returns the DKIM and Return-Path
// DNS records as outputs; publish them with your DNS provider and then trigger
// verification with a DomainVerification resource. Create never blocks on DNS.
type Domain struct{}

var (
	_ infer.CustomResource[DomainArgs, DomainState] = (*Domain)(nil)
	_ infer.CustomUpdate[DomainArgs, DomainState]   = (*Domain)(nil)
	_ infer.CustomRead[DomainArgs, DomainState]     = (*Domain)(nil)
	_ infer.CustomDelete[DomainState]               = (*Domain)(nil)
)

type DomainArgs struct {
	Name             string  `pulumi:"name" provider:"replaceOnChanges"`
	ReturnPathDomain *string `pulumi:"returnPathDomain,optional"`
}

type DomainState struct {
	DomainArgs
	DomainID int `pulumi:"domainId"`

	// DKIM
	DKIMVerified         bool   `pulumi:"dkimVerified"`
	WeakDKIM             bool   `pulumi:"weakDkim"`
	DKIMHost             string `pulumi:"dkimHost"`
	DKIMTextValue        string `pulumi:"dkimTextValue"`
	DKIMPendingHost      string `pulumi:"dkimPendingHost"`
	DKIMPendingTextValue string `pulumi:"dkimPendingTextValue"`
	DKIMRevokedHost      string `pulumi:"dkimRevokedHost"`
	DKIMRevokedTextValue string `pulumi:"dkimRevokedTextValue"`
	DKIMUpdateStatus     string `pulumi:"dkimUpdateStatus"`

	SafeToRemoveRevokedKeyFromDNS bool `pulumi:"safeToRemoveRevokedKeyFromDns"`

	// Return-Path
	ReturnPathDomainVerified   bool   `pulumi:"returnPathDomainVerified"`
	ReturnPathDomainCNAMEValue string `pulumi:"returnPathDomainCnameValue"`
}

func (a *DomainArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Name, "The domain name to send from (e.g. `example.com`). Immutable — changing it replaces the domain.")
	an.Describe(&a.ReturnPathDomain, "Custom Return-Path subdomain (e.g. `pm-bounces.example.com`). "+
		"Must be a subdomain of `name` and have a CNAME pointing at the value of `returnPathDomainCnameValue`.")
}

func (s *DomainState) Annotate(an infer.Annotator) {
	an.Describe(&s.DKIMPendingHost, "Hostname for the DKIM TXT record to publish (publish this first).")
	an.Describe(&s.DKIMPendingTextValue, "Value for the DKIM TXT record to publish.")
	an.Describe(&s.DKIMHost, "Hostname of the currently-active DKIM record (populated after verification).")
	an.Describe(&s.DKIMTextValue, "Value of the currently-active DKIM record.")
	an.Describe(&s.ReturnPathDomainCNAMEValue, "CNAME target for the Return-Path record (typically `pm.mtasv.net`).")
	an.Describe(&s.DKIMVerified, "Whether DKIM has been verified by Postmark.")
	an.Describe(&s.ReturnPathDomainVerified, "Whether the Return-Path CNAME has been verified by Postmark.")
}

func (*Domain) Create(ctx context.Context, req infer.CreateRequest[DomainArgs]) (infer.CreateResponse[DomainState], error) {
	if req.DryRun {
		return infer.CreateResponse[DomainState]{Output: DomainState{DomainArgs: req.Inputs}}, nil
	}
	client, err := accountClient(ctx)
	if err != nil {
		return infer.CreateResponse[DomainState]{}, err
	}
	d, err := client.CreateDomain(ctx, postmark.DomainCreateRequest{
		Name:             req.Inputs.Name,
		ReturnPathDomain: deref(req.Inputs.ReturnPathDomain, ""),
	})
	if err != nil {
		return infer.CreateResponse[DomainState]{}, fmt.Errorf("creating domain %q: %w", req.Inputs.Name, err)
	}
	return infer.CreateResponse[DomainState]{
		ID:     strconv.FormatInt(d.ID, 10),
		Output: domainStateFromAPI(d),
	}, nil
}

func (*Domain) Read(ctx context.Context, req infer.ReadRequest[DomainArgs, DomainState]) (infer.ReadResponse[DomainArgs, DomainState], error) {
	client, err := accountClient(ctx)
	if err != nil {
		return infer.ReadResponse[DomainArgs, DomainState]{}, err
	}
	id, err := idToInt64(req.ID)
	if err != nil {
		return infer.ReadResponse[DomainArgs, DomainState]{}, err
	}
	d, err := client.GetDomain(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return infer.ReadResponse[DomainArgs, DomainState]{}, nil
		}
		return infer.ReadResponse[DomainArgs, DomainState]{}, fmt.Errorf("reading domain %s: %w", req.ID, err)
	}
	state := domainStateFromAPI(d)
	return infer.ReadResponse[DomainArgs, DomainState]{
		ID:     req.ID,
		Inputs: state.DomainArgs,
		State:  state,
	}, nil
}

func (*Domain) Update(ctx context.Context, req infer.UpdateRequest[DomainArgs, DomainState]) (infer.UpdateResponse[DomainState], error) {
	if req.DryRun {
		out := req.State
		out.DomainArgs = req.Inputs
		return infer.UpdateResponse[DomainState]{Output: out}, nil
	}
	client, err := accountClient(ctx)
	if err != nil {
		return infer.UpdateResponse[DomainState]{}, err
	}
	id, err := idToInt64(req.ID)
	if err != nil {
		return infer.UpdateResponse[DomainState]{}, err
	}
	// Only ReturnPathDomain is editable; name changes force replacement.
	d, err := client.EditDomain(ctx, id, postmark.DomainEditRequest{
		ReturnPathDomain: deref(req.Inputs.ReturnPathDomain, ""),
	})
	if err != nil {
		return infer.UpdateResponse[DomainState]{}, fmt.Errorf("updating domain %s: %w", req.ID, err)
	}
	return infer.UpdateResponse[DomainState]{Output: domainStateFromAPI(d)}, nil
}

func (*Domain) Delete(ctx context.Context, req infer.DeleteRequest[DomainState]) (infer.DeleteResponse, error) {
	client, err := accountClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	id, err := idToInt64(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := client.DeleteDomain(ctx, id); err != nil {
		if isNotFound(err) {
			return infer.DeleteResponse{}, nil
		}
		return infer.DeleteResponse{}, fmt.Errorf("deleting domain %s: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

func domainStateFromAPI(d postmark.DomainDetails) DomainState {
	return DomainState{
		DomainArgs: DomainArgs{
			Name:             d.Name,
			ReturnPathDomain: ptr(d.ReturnPathDomain),
		},
		DomainID:                      int(d.ID),
		DKIMVerified:                  d.DKIMVerified,
		WeakDKIM:                      d.WeakDKIM,
		DKIMHost:                      d.DKIMHost,
		DKIMTextValue:                 d.DKIMTextValue,
		DKIMPendingHost:               d.DKIMPendingHost,
		DKIMPendingTextValue:          d.DKIMPendingTextValue,
		DKIMRevokedHost:               d.DKIMRevokedHost,
		DKIMRevokedTextValue:          d.DKIMRevokedTextValue,
		DKIMUpdateStatus:              d.DKIMUpdateStatus,
		SafeToRemoveRevokedKeyFromDNS: d.SafeToRemoveRevokedKeyFromDNS,
		ReturnPathDomainVerified:      d.ReturnPathDomainVerified,
		ReturnPathDomainCNAMEValue:    d.ReturnPathDomainCNAMEValue,
	}
}
