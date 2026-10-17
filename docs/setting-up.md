# Setting up a site

[Documentation](README.md) › Setting up a site

The pages to read, in order, to put an Anahita site on the web: for a school,
a newsroom, a library, a research group, a non-profit or any community of
your own. The web app is a separate repository, anahita-web; its pages are
linked where they come in.

1. [Getting started](getting-started.md): run the whole stack on your own
   machine first, and sign in to it. Then the web app's
   [Getting started](https://github.com/anahitasocial/anahita-web/blob/main/docs/getting-started.md).
2. [Configuration](configuration.md): every setting. The ones to decide
   before anybody signs up are the site's name
   ([Site identity](configuration.md#site-identity)), who may join
   ([Registration](configuration.md#registration)), what visitors may read
   ([Privacy of the installation](configuration.md#privacy-of-the-installation)),
   and the first super administrator
   ([First super administrator](configuration.md#first-super-administrator)).
3. [Registration and agreements](registration.md): how people join, the
   approval queue, invitations, and your Terms of Service and Privacy Policy.
4. [Privacy](privacy.md): who can see a post, a profile and the site, and the
   two settings that decide it for the whole installation.
5. [Deploying](deploying.md): a live site on Kubernetes, on any provider.
   Hostnames, certificates, object storage, and the database. The web app is
   built and hosted on its own:
   [Deploying the web app](https://github.com/anahitasocial/anahita-web/blob/main/docs/deploying.md).
6. [Federation](federation.md#setting-it-up): joining the fediverse and
   Bluesky. It is off until you turn it on, and each profile then chooses for
   itself. Choose the names first: they are for good.
7. The services a site may run or leave out. Each page says what happens
   without it:
   - [Videos](videos.md#running-it-and-not-running-it)
   - [Link previews](link-previews.md#running-it-and-not-running-it)
   - [Recommendations](recommendations.md#running-it-and-not-running-it)
   - [Download your data](data-export.md#running-it-and-not-running-it)
   - [The web shell](web-shell.md#running-it), which gives each shared link
     its own title and picture
   - [Websites](websites.md): a website's feed as a profile to follow

Next: [Making it your own](making-it-your-own.md), then
[Keeping it running](keeping-it-running.md).
