# A site in numbers (roadmap row 171)

Asked for by the user on 2026-11-22, after counting banu.io: 29,224 nodes
and 10,917 edges, about a thousand arriving a day, for nine people, on one
small server at about US$35 a month. A site like that, few people and much
passing through, is what most early sites running Anahita will be, and the
plain measure of it is what it handles for what it costs, not what it costs
for each person alone.

Two parts: a tool for the operator, then a public page any site may turn on.
The public page is for the public good: anybody can see what running a site
costs, and a site's people can see their share of it, the ground for sharing
the cost among them later, if a site wants to.

## Part 1: `make stats`, for the operator

In banu.io first, then the recipes: what the site holds (nodes, edges, file
storage), what it handled this month and the last (new nodes and edges, and
federation deliveries in and out), the server's memory and CPU in use, and
the month's cost, from the cloud's bill or from a figure the operator sets.
Then the measure: the cost of a thousand items handled, beside the headroom
left (memory, on banu.io: 79% in use against 9% of the CPU).

## Part 2: the numbers page

An endpoint on every site, and a quiet page in the web app, "This site in
numbers", from About. Who may see it is one setting for the whole page, from
the access levels a site has (the user, 2026-11-22):

| Setting | Who sees it |
| --- | --- |
| 1. Off | Nobody: no page, and the endpoint answers 404 (the default) |
| 2. Public | Anybody, signed in or not |
| 3. Members | Anybody with an account on the site |
| 4. Administrators | Administrators and super administrators |
| 5. Super administrators | Super administrators only |

Off until its operator chooses otherwise, and the cost shown only if they
give it: sharing is consented to.

| Shown | How |
| --- | --- |
| 1. Items handled, each month | Made here and arriving from elsewhere together; rounded |
| 2. Items held, and file storage | Rounded |
| 3. People, and active people | As NodeInfo publishes them already |
| 4. The monthly cost | As the operator gives it, if they do |
| 5. Cost per thousand items, and per person | Worked out from the above |
| 6. Headroom, and how fast pages answer | Memory and CPU in use; the slowest pages' time |

It keeps its own history, a line a month, so the trend is there to read and a
monthly screen capture is a picture of the page.

**Nothing that picks out anybody:**

1. No kinds: never posts, likes, follows or profiles counted apart, only
   items. On a site of nine people, three posts yesterday can point at one.
2. No days or hours: months, which blur anyone's habits.
3. Rounded, to the nearest hundred, or ten on a small site, so no one action
   shows.
4. No counts of people past what NodeInfo publishes.

anahita.io shows banu.io's, read at its nightly build, as a working example.

## Later

Sharing a site's cost among its people, on what the page shows: not planned,
and nothing is offered for sale.

## Status

1. Planned 2026-11-22, to follow row 170's bugs and quick wins.
2. **Part 1 built for banu.io 2026-11-22:** `make stats`
   (`deploy/stats.py` there). It counts the last 30 days, since what is still
   here is what is counted and older months are thinned by
   `REMOTE_UNFOLLOWED_DAYS`; and the cost from the last seven days of AWS's
   bill, a month of them, since a calendar month's carries the domains'
   yearly renewal. Its first figures: 29,252 nodes and 10,924 edges held;
   34,169 items handled in 30 days; US$41.32 a month; US$1.21 for a thousand
   items; memory 78% in use, CPU 10%. The recipes' version, and the public
   page, wait.
