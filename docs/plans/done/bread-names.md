# BREAD names in the services' code (roadmap row 127)

Asked for by the user on 2026-11-07, after anahita-web's code and addresses
were given BREAD names the same day: "we would want to stick to BREAD as much
as possible." The rule is in [AGENTS.md](../../../AGENTS.md#code), rule 5:
**Browse, Read, Edit, Add, Delete**. Planned for later the same day, and
done that day, for v1.0.17.

## Contents

1. [What there is now](#what-there-is-now)
2. [Renamed](#renamed)
3. [Kept, with why](#kept-with-why)
4. [gRPC, without a broken deploy](#grpc-without-a-broken-deploy)
5. [Checking](#checking)

## What there is now

Found on 2026-11-07. Of the services' own functions, 263 begin with `Read`,
182 with `Browse`, 181 with `Add`, 178 with `Delete` and 107 with `Edit`.
Thirty are named otherwise, in seven services and anahita-libs, besides
their tests. No HTTP address is named otherwise.

## Renamed

`List` becomes `Browse`, `Remove` becomes `Delete`, and `Update` becomes
`Edit`; the rest of each name stays.

| Where | Now | Becomes |
| --- | --- | --- |
| abuse-report-service | `cases.RemoveReportsBy` | `DeleteReportsBy` |
| actor-service | `groupHandler.RemoveAsSpam`, `personHandler.RemoveAsSpam` | `DeleteAsSpam` |
| actor-service | `personHandler.RemoveTag` | `DeleteTag` |
| anahita-libs | `EventsGRPC.ListFor`, `ListHostedBy`, `ListOpen` | `BrowseFor`, `BrowseHostedBy`, `BrowseOpen` |
| anahita-libs | `MediaGRPC.ListAuthored` | `BrowseAuthored` |
| anahita-libs | `Keeper.RemovePost`, `RemovePictures`, `RemoveOldPictures` | `DeletePost`, `DeletePictures`, `DeleteOldPictures` |
| auth-service | `authLogs.UpdateOAuthRefreshTokenID` | `EditOAuthRefreshTokenID` |
| graph-grpc-service | `eventsRepo.ListFor`, `ListHostedBy`, `ListOpen` | `BrowseFor`, `BrowseHostedBy`, `BrowseOpen` |
| graph-grpc-service | `invitesRepo.ListForActor`, `ListForInvitee` | `BrowseForActor`, `BrowseForInvitee` |
| graph-grpc-service | `mediumHandler.ListAuthored`, `ListCitedBy`, `ListQuotes`, `ListSaved` | `BrowseAuthored`, `BrowseCitedBy`, `BrowseQuotes`, `BrowseSaved` |
| graph-grpc-service | `LikesRelRepo.UpdateLikeCount`, `RepostsRepo.UpdateRepostCount`, `SubscriptionsRelRepo.UpdateSubscriberCount`, `socialGraphCountsRepo.UpdateFollowerCount`, `UpdateLeaderCount` | `EditLikeCount`, `EditRepostCount`, `EditSubscriberCount`, `EditFollowerCount`, `EditLeaderCount` |
| identity-image-service | `imageHandler.RemoveAvatar`, `RemoveCover` | `DeleteAvatar`, `DeleteCover` |

What people read stays as it is: "Remove as spam" is still the app's label.

## Kept, with why

1. **`NewX`,** Go's constructors, and **`GetX`,** the getters protobuf
   makes: Go's forms, not ours.
2. **atproto-service's `Client.CreateRecord` and `CreateSession`:** the AT
   Protocol's own methods (`com.atproto.repo.createRecord`,
   `com.atproto.server.createSession`), named as Bluesky's docs name them.

## gRPC, without a broken deploy

Five of the renames are gRPC methods (`anahita-libs/proto/graphpb`):
`MediumService.ListQuotes`, `ListSaved`, `ListCitedBy`, `ListAuthored`, and
the events service's `List`. Each becomes `Browse…` (`Browse` for events),
with its request and response messages renamed to match
(`QuotesListRequest` to `QuotesBrowseRequest`, and so on), and the Go code
made again from the `.proto` files.

During a rolling deploy, a service still running the old code would call a
method the new graph-grpc-service no longer serves, and be answered
`Unimplemented`. So for one release graph-grpc-service serves both names,
the old ones calling the new, and the old ones are removed in the release
after. The release notes say so.

## Checking

1. `go build ./...` and `go vet ./...` in every service, and the tests of
   each one touched, against a database where they use one.
2. The docs grepped for every old name (`docs/data-export.md` names `ListAuthored`),
   and each one changed.
3. On the development stack, the pages that use the renamed calls: a
   profile's events, an event's invitations, a post's quotes, saved posts,
   what a post is cited by, removing a picture and a profile photo.

## Done

2026-11-07. The thirty renamed with gopls, each interface with what
implements it; the five gRPC methods under their new names, the old ones
served beside them in `graph-grpc-service/cmd/server/handlers/old_names.go`
until the release after v1.0.17; the API reference made again, where only
the handlers' names changed. Every module built and vetted, and the tests
of each one touched passed. On the development stack, the saved posts, a
post's quotes and what cites it, and the events lists answered through the
new names.
