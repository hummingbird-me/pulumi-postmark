package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mrz1836/postmark"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// SenderSignature is a single verified From address. Postmark emails the address
// a confirmation link on creation; a human must click it. The provider never
// blocks on confirmation: `confirmed` is a read-only output that flips to true
// (visible after a `pulumi refresh`) once the link is clicked. Prefer a verified
// Domain where possible, since it needs no human step.
type SenderSignature struct{}

var (
	_ infer.CustomResource[SenderSignatureArgs, SenderSignatureState] = (*SenderSignature)(nil)
	_ infer.CustomUpdate[SenderSignatureArgs, SenderSignatureState]   = (*SenderSignature)(nil)
	_ infer.CustomRead[SenderSignatureArgs, SenderSignatureState]     = (*SenderSignature)(nil)
	_ infer.CustomDelete[SenderSignatureState]                        = (*SenderSignature)(nil)
)

type SenderSignatureArgs struct {
	FromEmail                string  `pulumi:"fromEmail" provider:"replaceOnChanges"`
	Name                     string  `pulumi:"name"`
	ReplyToEmail             *string `pulumi:"replyToEmail,optional"`
	ReturnPathDomain         *string `pulumi:"returnPathDomain,optional"`
	ConfirmationPersonalNote *string `pulumi:"confirmationPersonalNote,optional"`
	TriggerResend            *string `pulumi:"triggerResend,optional"`
}

type SenderSignatureState struct {
	SenderSignatureArgs
	SignatureID  int    `pulumi:"signatureId"`
	Domain       string `pulumi:"domain"`
	EmailAddress string `pulumi:"emailAddress"`
	Confirmed    bool   `pulumi:"confirmed"`

	// DKIM / Return-Path status (inherited from the address's domain).
	DKIMVerified               bool   `pulumi:"dkimVerified"`
	DKIMHost                   string `pulumi:"dkimHost"`
	DKIMTextValue              string `pulumi:"dkimTextValue"`
	DKIMPendingHost            string `pulumi:"dkimPendingHost"`
	DKIMPendingTextValue       string `pulumi:"dkimPendingTextValue"`
	ReturnPathDomainVerified   bool   `pulumi:"returnPathDomainVerified"`
	ReturnPathDomainCNAMEValue string `pulumi:"returnPathDomainCnameValue"`
}

func (a *SenderSignatureArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.FromEmail, "The email address to sign. Immutable — changing it replaces the signature.")
	an.Describe(&a.Name, "From name associated with the signature.")
	an.Describe(&a.ReplyToEmail, "Optional Reply-To email address.")
	an.Describe(&a.ReturnPathDomain, "Optional custom Return-Path subdomain of the From address's domain.")
	an.Describe(&a.ConfirmationPersonalNote, "Optional note (max 400 chars) included in the confirmation email.")
	an.Describe(&a.TriggerResend, "Arbitrary value; change it to resend the confirmation email on the next update.")
}

func (s *SenderSignatureState) Annotate(an infer.Annotator) {
	an.Describe(&s.Confirmed, "Whether the signature has been confirmed. Postmark requires a human to click "+
		"the confirmation email; run `pulumi refresh` to observe this flipping to true.")
}

func (*SenderSignature) Create(ctx context.Context, req infer.CreateRequest[SenderSignatureArgs]) (infer.CreateResponse[SenderSignatureState], error) {
	if req.DryRun {
		return infer.CreateResponse[SenderSignatureState]{Output: SenderSignatureState{SenderSignatureArgs: req.Inputs}}, nil
	}
	client, err := accountClient(ctx)
	if err != nil {
		return infer.CreateResponse[SenderSignatureState]{}, err
	}
	d, err := client.CreateSenderSignature(ctx, postmark.SenderSignatureCreateRequest{
		FromEmail:                req.Inputs.FromEmail,
		Name:                     req.Inputs.Name,
		ReplyToEmail:             deref(req.Inputs.ReplyToEmail, ""),
		ReturnPathDomain:         deref(req.Inputs.ReturnPathDomain, ""),
		ConfirmationPersonalNote: deref(req.Inputs.ConfirmationPersonalNote, ""),
	})
	if err != nil {
		return infer.CreateResponse[SenderSignatureState]{}, fmt.Errorf("creating sender signature %q: %w", req.Inputs.FromEmail, err)
	}
	return infer.CreateResponse[SenderSignatureState]{
		ID:     strconv.FormatInt(d.ID, 10),
		Output: signatureStateFromAPI(d, req.Inputs.TriggerResend),
	}, nil
}

func (*SenderSignature) Read(ctx context.Context, req infer.ReadRequest[SenderSignatureArgs, SenderSignatureState]) (infer.ReadResponse[SenderSignatureArgs, SenderSignatureState], error) {
	client, err := accountClient(ctx)
	if err != nil {
		return infer.ReadResponse[SenderSignatureArgs, SenderSignatureState]{}, err
	}
	id, err := idToInt64(req.ID)
	if err != nil {
		return infer.ReadResponse[SenderSignatureArgs, SenderSignatureState]{}, err
	}
	d, err := client.GetSenderSignature(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return infer.ReadResponse[SenderSignatureArgs, SenderSignatureState]{}, nil
		}
		return infer.ReadResponse[SenderSignatureArgs, SenderSignatureState]{}, fmt.Errorf("reading sender signature %s: %w", req.ID, err)
	}
	state := signatureStateFromAPI(d, req.Inputs.TriggerResend)
	return infer.ReadResponse[SenderSignatureArgs, SenderSignatureState]{
		ID:     req.ID,
		Inputs: state.SenderSignatureArgs,
		State:  state,
	}, nil
}

func (*SenderSignature) Update(ctx context.Context, req infer.UpdateRequest[SenderSignatureArgs, SenderSignatureState]) (infer.UpdateResponse[SenderSignatureState], error) {
	if req.DryRun {
		out := req.State
		out.SenderSignatureArgs = req.Inputs
		return infer.UpdateResponse[SenderSignatureState]{Output: out}, nil
	}
	client, err := accountClient(ctx)
	if err != nil {
		return infer.UpdateResponse[SenderSignatureState]{}, err
	}
	id, err := idToInt64(req.ID)
	if err != nil {
		return infer.UpdateResponse[SenderSignatureState]{}, err
	}
	d, err := client.EditSenderSignature(ctx, id, postmark.SenderSignatureEditRequest{
		Name:                     req.Inputs.Name,
		ReplyToEmail:             deref(req.Inputs.ReplyToEmail, ""),
		ReturnPathDomain:         deref(req.Inputs.ReturnPathDomain, ""),
		ConfirmationPersonalNote: deref(req.Inputs.ConfirmationPersonalNote, ""),
	})
	if err != nil {
		return infer.UpdateResponse[SenderSignatureState]{}, fmt.Errorf("updating sender signature %s: %w", req.ID, err)
	}

	// Resend the confirmation email if the trigger nonce changed.
	if deref(req.Inputs.TriggerResend, "") != deref(req.State.TriggerResend, "") {
		if err := client.ResendSenderSignatureConfirmation(ctx, id); err != nil {
			return infer.UpdateResponse[SenderSignatureState]{}, fmt.Errorf("resending confirmation for sender signature %s: %w", req.ID, err)
		}
	}
	return infer.UpdateResponse[SenderSignatureState]{Output: signatureStateFromAPI(d, req.Inputs.TriggerResend)}, nil
}

func (*SenderSignature) Delete(ctx context.Context, req infer.DeleteRequest[SenderSignatureState]) (infer.DeleteResponse, error) {
	client, err := accountClient(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	id, err := idToInt64(req.ID)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if err := client.DeleteSenderSignature(ctx, id); err != nil {
		if isNotFound(err) {
			return infer.DeleteResponse{}, nil
		}
		return infer.DeleteResponse{}, fmt.Errorf("deleting sender signature %s: %w", req.ID, err)
	}
	return infer.DeleteResponse{}, nil
}

func signatureStateFromAPI(d postmark.SenderSignatureDetails, triggerResend *string) SenderSignatureState {
	return SenderSignatureState{
		SenderSignatureArgs: SenderSignatureArgs{
			FromEmail:                d.FromEmail,
			Name:                     d.Name,
			ReplyToEmail:             ptr(d.ReplyToEmail),
			ReturnPathDomain:         ptr(d.ReturnPathDomain),
			ConfirmationPersonalNote: ptr(d.ConfirmationPersonalNote),
			TriggerResend:            triggerResend,
		},
		SignatureID:                int(d.ID),
		Domain:                     d.Domain,
		EmailAddress:               d.FromEmail,
		Confirmed:                  d.Confirmed,
		DKIMVerified:               d.DKIMVerified,
		DKIMHost:                   d.DKIMHost,
		DKIMTextValue:              d.DKIMTextValue,
		DKIMPendingHost:            d.DKIMPendingHost,
		DKIMPendingTextValue:       d.DKIMPendingTextValue,
		ReturnPathDomainVerified:   d.ReturnPathDomainVerified,
		ReturnPathDomainCNAMEValue: d.ReturnPathDomainCNAMEValue,
	}
}
