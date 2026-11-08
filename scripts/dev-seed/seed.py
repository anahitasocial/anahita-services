#!/usr/bin/env python3
"""A development stack's made-up site: about sixty people, twelve from each of
five shows as Faker names them (Star Trek, Stargate, Doctor Who, The Expanse and
The IT Crowd), their groups, events and locations, and who follows, posts,
replies, likes and reposts.

    make dev_seed

Run from the root of the repository, with the stack up (`make up`). The people
are written straight to the database, as sign-up would leave them, so that no
mail is sent; all the rest goes through the API, each person signed in as
themselves, so that counts, notifications and recommendations happen as they do
in use. It uses a fixed seed: run on the same stack it makes the same site.

The graph is meant to be even, as a real community is and as a database with one
big poster is not: people follow mostly inside their groups, with a few links
between them, a few people are followed widely and none by everybody, and what
each person posts runs from a little to a lot. The last thing printed is how
even it came out. Nothing here is for a live site, and it refuses to run on one
that does not say it is for development (`ENV=dev` in `.env`).
"""
import argparse, base64, hashlib, html, http.cookiejar, json, os, random, re, secrets, statistics
import subprocess, sys, threading, time, urllib.error, urllib.parse, urllib.request, uuid
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timedelta

parser = argparse.ArgumentParser()
parser.add_argument('--people', type=int, default=60)
parser.add_argument('--site', default='http://localhost')
parser.add_argument('--redirect', default='http://localhost:3000/oauth/callback')
args = parser.parse_args()

SITE = args.site
CLIENT = 'anahita-web'
# The web app's own, in src/constants/auth.js.
SCOPES = ('openid profile email offline_access actors:read actors:write '
          'media:read media:write likes:write socialgraph:read socialgraph:write '
          'notifications:read notifications:write taxonomy:write account:read '
          'account:write oauth:read oauth:write')
PERSON = 'node.actor.person-service.person.v1'
NOTE = 'node.medium.text-service.note.v1'
FOLLOW = 'edge.actor.follow.v1'
ADMINISTRATORS = ('jeanluc', 'samcarter', 'chrisjen')
now = datetime.utcnow().replace(microsecond=0)
rng = random.Random(1701)

# Who there is, by the group they are mostly in: twelve from each of five
# shows, by the names Faker gives them (faker-ruby/faker, tv_shows). Their
# names and places only: nothing here quotes the shows.
CREWS = {
    'USS Enterprise': 'jean-luc picard, william riker, data soong, geordi laforge, beverly crusher, deanna troi, benjamin sisko, kira nerys, jadzia dax, kathryn janeway, seven annika, odo ide',
    'Stargate Command': 'jack oneill, tealc chulak, daniel jackson, sam carter, janet frasier, george hammond, jonas quinn, cameron mitchell, vala maldoran, jacob carter, cassandra frasier, bratac chulak',
    'The TARDIS': 'rose tyler, adam mitchell, mickey smith, donna noble, martha jones, astrid peth, sarah-jane smith, jackson lake, christina desouza, adelaide brooke, wilfred mott, amy pond',
    'The Rocinante': 'jim holden, alex kamal, naomi nagata, amos burton, chrisjen avasarala, joe miller, roberta draper, julie mao, camina drummer, klaes ashford, anderson dawes, praxidike meng',
    'Reynholm Industries IT': 'roy trenneman, maurice moss, jen barber, douglas reynholm, denholm reynholm, richmond avenal, barbara reynholm, victoria reynholm, derek pippen, helen buley, april shephard, harold tong',
}
GROUPS = {
    'USS Enterprise': 'Its crew, and everybody who served on it.',
    'Stargate Command': 'Mission reports, gate addresses and the infirmary rota.',
    'The TARDIS': 'Travellers, past and future, comparing notes.',
    'The Rocinante': 'A small ship, its crew and the Belt around them.',
    'Reynholm Industries IT': 'The help desk in the basement, and its tickets.',
    'Stellar Cartography': 'Maps, surveys and things that are not on the maps yet.',
    'Board Game Night': 'Every other Friday, whoever you are.',
    'The Reading Room': 'What we are reading, and what we thought of it.',
}
PLACES = [
    ('Stargate Command', 'United States', 38.7445, -104.8461), ('Abydos', 'Abydos', 26.18, 31.92),
    ('Chulak', 'Chulak', 30.1, 31.3), ('Ceres Station', 'Ceres', -12.0, 130.0),
    ('Tycho Station', 'Tycho', -43.3, -11.2), ('Reynholm Industries', 'United Kingdom', 51.5155, -0.0922),
    ('Bajor', 'Bajor', 44.0, 11.0), ('Vulcan', 'Vulcan', 21.3, 39.8),
    ('Bozeman', 'United States', 45.6770, -111.0429), ('Paris', 'France', 48.8566, 2.3522),
]
EVENTS = [
    ('Gate room open day', 'Stargate Command', 'A look at the gate, and the long walk up the ramp.'),
    ('Archaeology evening', 'Abydos', 'Cartouches, glyphs and what they might mean.'),
    ('Ceres trade fair', 'Ceres Station', 'Water, air and everything else that is shipped.'),
    ('Drive maintenance workshop', 'Tycho Station', 'Bring your questions about engines.'),
    ('Help desk drop-in', 'Reynholm Industries', 'Bring a broken laptop, leave with a working one.'),
    ('Warp theory seminar', 'Bozeman', 'Field coils, for beginners.'),
    ('Peer review clinic', 'Vulcan', 'Bring your draft. It will be read closely.'),
    ('Star party', 'Paris', 'Telescopes on the roof, weather permitting.'),
]
TOPICS = {
    'USS Enterprise': ['the warp core alignment', 'a diplomatic dinner', 'the morning shift report', 'the holodeck queue', 'a new sensor calibration', 'the arboretum'],
    'Stargate Command': ['a new set of gate addresses', 'the translation of a cartouche', 'the off-world checklist', 'the infirmary rota', 'a mission report', 'the naquadah samples'],
    'The TARDIS': ['the console room', 'a trip to the past', 'the wardrobe', 'a very old library', 'the manual nobody reads', 'a picnic on a moon'],
    'The Rocinante': ['the coffee machine', 'a burn schedule', 'the water ration', 'the airlock checks', 'the Belt traffic', 'the galley'],
    'Reynholm Industries IT': ['the server room', 'a printer that will not print', 'the ticket queue', 'a password reset', 'the backup tapes', 'the new office chairs'],
}
SHAPES = [
    'Spent the morning on {t}. Better than expected. #{h}',
    'A thought about {t}: nobody writes down why, only what. #{h}',
    'If you are near the {g} this week, ask me about {t}.',
    'Still thinking about {t}. Has anybody tried doing it the other way round? #{h}',
    'Notes on {t}, for whoever comes after me. #{h} #notes',
    'Three things I got wrong about {t}, and one I got right. #{h}',
    '{T} went well today. Thanks to everybody who helped.',
    'Old notes about {t} turned up. They hold up better than I do. #{h}',
    'A short question about {t}. What would you change? #{h}',
    'Quiet day. Read about {t} and made tea.',
]
REPLIES = ['Agreed, and well put.', 'I saw that too. Worth a longer look.', 'Not how I remember it.', 'Thanks for writing this down.',
           'Could you say more about the second part?', 'This is why I read this every morning.', 'I would do it the other way round.',
           'Well spotted.', 'Sharing this with the others.', 'Remind me to buy you a coffee.', 'Interesting.', 'Good to hear.']
# Pronouns, by handle, as a community has them: she, he and they, and some
# who have said none. Whoever is not named here is he/him.
PRONOUNS = {
    'she/her': 'beverly deanna kira kathryn janet samcarter vala cassandra rose donna martha astrid sarahjane christina '
               'adelaide amypond naomi chrisjen roberta julie jenbarber barbara victoria helen april',
    'they/them': 'data odoide jadzia seven tealc jackson alex camina maurice',
    '': 'george wilfred klaes harold',
}
PRONOUN_OF = {alias: said for said, aliases in PRONOUNS.items() for alias in aliases.split()}
HASHTAGS = {'USS Enterprise': 'engineering', 'Stargate Command': 'exploration', 'The TARDIS': 'timetravel',
            'The Rocinante': 'thebelt', 'Reynholm Industries IT': 'helpdesk'}


def setting(name):
    for line in open('.env'):
        if line.startswith(name + '='):
            return line.split('=', 1)[1].strip().strip('"')
    return ''


# Everybody's password: DEV_SEED_PASSWORD in .env, so that none is published.
# A stack may be open to the internet (scripts/funnel.sh).
PASSWORD = setting('DEV_SEED_PASSWORD')


def hashed(password):
    out = subprocess.run(['htpasswd', '-bnBC', '4', '', password], capture_output=True)
    if out.returncode:
        sys.exit('htpasswd is needed, to hash the password: apache2-utils on Linux.')
    return out.stdout.decode().strip().strip(':').replace('$2y$', '$2a$', 1)


def compose(*a, stdin=None):
    out = subprocess.run(['docker-compose', *a], input=stdin, capture_output=True)
    if out.returncode:
        sys.exit(f'docker-compose {" ".join(a[:3])}: {out.stderr.decode()[-500:]}')
    return out.stdout.decode()


def mysql(sql):
    out = compose('exec', '-T', 'mysql', 'sh', '-c', 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" -N anahita', stdin=sql.encode())
    return [line.split('\t') for line in out.strip().split('\n') if line]


def q(value):
    return "'" + str(value).replace('\\', '\\\\').replace("'", "''") + "'"


# --- Signing in, as the web app does.

class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *a, **k): return None


class Person:
    def __init__(self, id, alias, name, crew):
        self.id, self.alias, self.name, self.crew = id, alias, name, crew
        self.headers = None
        self.lock = threading.Lock()
        self.attract = 1.0

    def raw(self, method, url, body=None, headers=None):
        req = urllib.request.Request(url, data=body, method=method, headers=headers or {})
        try:
            resp = self.opener.open(req, timeout=60)
            return resp.status, resp.headers, resp.read()
        except urllib.error.HTTPError as e:
            return e.code, e.headers, e.read()

    def signin(self):
        # From nothing each time: an access token lasts a quarter of an hour.
        self.opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()), NoRedirect)
        form = lambda d: urllib.parse.urlencode(d).encode()
        F = {'Content-Type': 'application/x-www-form-urlencoded'}
        csrf = lambda b: html.unescape(re.search(r'name="csrf_token" value="([^"]+)"', b.decode()).group(1))
        verifier = secrets.token_urlsafe(48)
        challenge = base64.urlsafe_b64encode(hashlib.sha256(verifier.encode()).digest()).rstrip(b'=').decode()
        authorize = f'{SITE}/oauth/authorize?' + urllib.parse.urlencode({
            'response_type': 'code', 'client_id': CLIENT, 'redirect_uri': args.redirect, 'scope': SCOPES,
            'state': 'seed', 'code_challenge': challenge, 'code_challenge_method': 'S256'})
        self.raw('GET', authorize)
        st, h, b = self.raw('GET', f'{SITE}/oauth/login')
        st, h, b = self.raw('POST', f'{SITE}/oauth/login', form({
            'identifier': f'{self.alias}@example.org', 'password': PASSWORD, 'csrf_token': csrf(b)}), F)
        if b'Too many' in b:
            return False
        location = h.get('Location', '')
        for _ in range(6):
            if 'code=' in location:
                break
            st, h, b = self.raw('GET', urllib.parse.urljoin(SITE, location) if location else authorize)
            location = h.get('Location', '')
        if 'code=' not in location:
            sys.exit(f'@{self.alias} could not sign in ({st}): {b[:300]}')
        code = urllib.parse.parse_qs(urllib.parse.urlparse(location).query)['code'][0]
        st, h, b = self.raw('POST', f'{SITE}/oauth/token', form({
            'grant_type': 'authorization_code', 'client_id': CLIENT, 'code': code,
            'redirect_uri': args.redirect, 'code_verifier': verifier}), F)
        self.headers = {'Authorization': 'Bearer ' + json.loads(b)['access_token'], 'Content-Type': 'application/json'}
        return True

    def api(self, method, path, body=None, form=None, ok=(200, 201, 202, 204)):
        if self.headers is None:
            signin(self)
        data, headers = None, dict(self.headers)
        if form is not None:
            data, headers['Content-Type'] = urllib.parse.urlencode(form).encode(), 'application/x-www-form-urlencoded'
        elif body is not None:
            data = json.dumps(body).encode()
        st, h, b = self.raw(method, SITE + path, data, headers)
        # A development database has few connections, and the services share them.
        for wait in (1, 3, 8, 20):
            if st < 500:
                break
            time.sleep(wait)
            st, h, b = self.raw(method, SITE + path, data, headers)
        if st == 401:
            with self.lock:
                if self.headers == headers or self.headers['Authorization'] == headers['Authorization']:
                    signin(self)
            return self.api(method, path, body, form, ok)
        if st not in ok:
            raise RuntimeError(f'{method} {path} as @{self.alias}: {st} {b[:300]}')
        try:
            answer = json.loads(b) if b else {}
        except ValueError:
            return {}
        return answer.get('data', answer) if isinstance(answer, dict) else answer


def signin(person):
    """A site allows fifty sign-ins from one address in fifteen minutes. Here
    there is one address, and these are not an attack: its counter is cleared
    and it goes on."""
    while not person.signin():
        for key in compose('exec', '-T', 'redis', 'redis-cli', '--scan', '--pattern', 'ratelimit:login_ip:*').split():
            compose('exec', '-T', 'redis', 'redis-cli', 'DEL', key)


def react(person, method, path, body=None):
    """A like, a reply or a repost. Refused while a service is short of
    connections, it is tried again, then left out: nobody will miss one."""
    for wait in (0, 2, 6):
        time.sleep(wait)
        try:
            return person.api(method, path, body, ok=(200, 201, 204, 409))
        except RuntimeError:
            pass
    return None


def together(jobs):
    """Each (function, *arguments), four at a time, stopping at the first
    refusal that is not expected."""
    jobs = list(jobs)
    with ThreadPoolExecutor(4) as pool:
        return list(pool.map(lambda job: job[0](*job[1:]), jobs))


# --- The people.

def people():
    wanted = []
    for crew, names in CREWS.items():
        for name in names.split(', '):
            wanted.append((crew, name))
    rng.shuffle(wanted)
    wanted = wanted[:args.people]
    password = hashed(PASSWORD)
    if mysql("SELECT COUNT(*) FROM nodes WHERE alias = 'jeanluc'")[0][0] != '0':
        sys.exit('There are seeded people here already. Build the stack again first.')
    made = []
    for crew, full in wanted:
        given, family = full.split(' ', 1)
        alias = given.replace('-', '') if len(given) > 3 else given + family
        name = f'{given.title()} {family.title()}'
        made.append((crew, alias, name))
    sql = []
    for crew, alias, name in made:
        sql.append(
            f"INSERT INTO nodes (uuid, type, service, name, alias, body, owner_type, access, enabled, person_type, person_pronouns, created_at, updated_at) "
            f"VALUES ({q(uuid.uuid4())}, {q(PERSON)}, 'person-service', {q(name)}, {q(alias)}, {q('Of ' + crew + '.')}, '', 'public', 1, 'registered', "
            f"{q(PRONOUN_OF.get(alias, 'he/him')) if PRONOUN_OF.get(alias, 'he/him') else 'NULL'}, "
            f"{q(now - timedelta(days=rng.randint(60, 1400)))}, {q(now)});\n"
            f"INSERT INTO auth_accounts (node_id, email, password, email_verified_at, onboarded_at, tos_accepted, tos_accepted_version, "
            f"pp_accepted, pp_accepted_version, rules_accepted, rules_accepted_version) "
            f"SELECT LAST_INSERT_ID(), {q(alias + '@example.org')}, {q(password)}, NOW(), NOW(), 1, a.tos_accepted_version, 1, a.pp_accepted_version, 1, a.rules_accepted_version "
            f"FROM auth_accounts a WHERE a.node_id = (SELECT MIN(node_id) FROM auth_accounts);\n")
    mysql(''.join(sql))
    rows = mysql('SELECT id, alias, name FROM nodes WHERE alias IN (' + ','.join(q(m[1]) for m in made) + ')')
    crew_of = {m[1]: m[0] for m in made}
    return [Person(int(r[0]), r[1], r[2], crew_of[r[1]]) for r in rows]


def main():
    if setting('ENV') != 'dev':
        sys.exit('This is for a development stack: .env does not say ENV=dev.')
    if len(PASSWORD) < 16:
        sys.exit('Put a password of 16 characters or more in .env as DEV_SEED_PASSWORD.')

    # Each post fans out through every service at once. Until MySQL restarts.
    mysql('SET GLOBAL max_connections = 800')
    crew = people()
    # Groups and locations are made by administrators, as a site has it unless
    # it says otherwise (GROUPS_FROM, LOCATIONS_FROM).
    admins = [p for p in crew if p.alias in ADMINISTRATORS] or crew[:3]
    mysql(f"UPDATE nodes SET person_type = 'administrator' WHERE id IN ({','.join(str(p.id) for p in admins)})")
    print(f'{len(crew)} people, {len(admins)} administrators.')
    for p in crew:
        signin(p)
    print('Signed in.')

    # A few are followed widely, most by few: a long tail with no one at its head.
    for p in crew:
        p.attract = min(rng.paretovariate(1.4), 8.0)

    by_crew = {}
    for p in crew:
        by_crew.setdefault(p.crew, []).append(p)

    # Groups, each founded by somebody in it.
    groups = {}
    for name, about in GROUPS.items():
        founder = rng.choice(admins)
        made = founder.api('POST', '/groups/', form={'name': name, 'body': about, 'website_url': ''})
        groups[name] = (int(made['id']), founder)
        # A group begins closed to all but its administrators.
        founder.api('PATCH', f"/groups/{made['id']}/access", form={'access': 'public'})
    print(f'{len(groups)} groups.')

    # Locations and events.
    places = {}
    for name, country, lat, lon in PLACES:
        made = rng.choice(admins).api('POST', '/locations/', form={'name': name, 'country': country, 'latitude': lat, 'longitude': lon})
        places[name] = int(made['id'])
    events, over = [], []
    for i, (name, place, about) in enumerate(EVENTS):
        host = rng.choice(crew)
        days = rng.choice([-120, -60, -14, 7, 14, 30, 60, 90]) + i
        start = now + timedelta(days=abs(days))
        made = host.api('POST', '/events/', {
            'name': name, 'body': about, 'city': place, 'timezone_name': 'America/Vancouver',
            'starts_at': start.strftime('%Y-%m-%dT%H:%M:%SZ'), 'ends_at': (start + timedelta(hours=3)).strftime('%Y-%m-%dT%H:%M:%SZ')})
        events.append((int(made['id']), host))
        if days < 0:
            over.append((events[-1][0], 2 * abs(days)))
    print(f'{len(places)} locations, {len(events)} events.')

    # Who follows whom: mostly inside a crew, some across, weighted by attraction.
    edges = set()
    for p in crew:
        for other in by_crew[p.crew]:
            if other is not p and rng.random() < 0.65:
                edges.add((p, other))
        outside = [o for o in crew if o.crew != p.crew]
        wanted = max(2, int(rng.lognormvariate(1.6, 0.7)))
        for other in rng.choices(outside, weights=[o.attract for o in outside], k=wanted):
            edges.add((p, other))
        if rng.random() < 0.5:
            for other in by_crew[p.crew]:
                if (other, p) in edges and rng.random() < 0.5:
                    edges.add((p, other))
    together((lambda a, b: a.api('POST', f'/socialgraph/{b.id}/followers/{a.id}', ok=(200, 201, 204, 409)), a, b) for a, b in edges)
    print(f'{len(edges)} follows.')

    # The real profiles and the websites here are followed as anybody is.
    others = [int(r[0]) for r in mysql(
        f"SELECT id FROM nodes WHERE deleted_at IS NULL AND purged_at IS NULL AND uri IS NULL AND ("
        f"(type = {q(PERSON)} AND id NOT IN ({','.join(str(p.id) for p in crew)})) OR type = 'node.actor.actor-service.website.v1')")]
    jobs = []
    for target in others:
        for p in rng.sample(crew, min(len(crew), rng.randint(4, 14))):
            jobs.append((lambda a, b: a.api('POST', f'/socialgraph/{b}/followers/{a.id}', ok=(200, 201, 204, 409)), p, target))
    together(jobs)
    print(f'{len(jobs)} follows of the profiles already here.')

    # Into groups, then events.
    jobs = []
    for name, (gid, founder) in groups.items():
        members = by_crew.get(name, []) + rng.sample(crew, rng.randint(6, 16))
        for p in {m.id: m for m in members}.values():
            if p is not founder:
                jobs.append((lambda a, g: a.api('POST', f'/socialgraph/{g}/followers/{a.id}', ok=(200, 201, 204, 409)), p, gid))
    together(jobs)
    jobs = []
    for eid, host in events:
        for p in rng.sample(crew, rng.randint(5, 18)):
            if p is not host:
                jobs.append((lambda a, e: a.api('PUT', f'/events/{e}/rsvp', {'rsvp': rng.choice(['going', 'going', 'maybe'])}), p, eid))
    together(jobs)
    print(f'{len(jobs)} going to events.')
    # An event cannot be made already over: a past one is moved there now.
    for eid, days in over:
        mysql(f'UPDATE nodes SET starts_at = starts_at - INTERVAL {days} DAY, ends_at = ends_at - INTERVAL {days} DAY WHERE id = {eid}')

    # Posts: from a little to a lot, nobody more than a few per cent of the whole.
    notes = []
    def post(p, owner, body, place):
        fields = [('content', body), ('access', 'public')] + ([('location_ids', place)] if place else [])
        made = p.api('POST', f'/notes/{owner}/', form=fields)
        notes.append((int(made['id']), p))
    jobs = []
    for p in crew:
        count = min(int(rng.lognormvariate(3.1, 0.8)) + 2, 70)
        topics = TOPICS[p.crew]
        for _ in range(count):
            t = rng.choice(topics)
            body = rng.choice(SHAPES).format(t=t, T=t[0].upper() + t[1:], g=p.crew, h=HASHTAGS[p.crew])
            owner = p.id
            if rng.random() < 0.2:
                gid, _f = groups.get(p.crew, groups['Board Game Night'])
                owner = gid
            place = places[rng.choice(list(places))] if rng.random() < 0.08 else None
            jobs.append((post, p, owner, body, place))
    rng.shuffle(jobs)
    together(jobs)
    print(f'{len(notes)} posts.')

    # What people do with them, mostly among those they follow.
    following = {}
    for a, b in edges:
        following.setdefault(a.id, []).append(b.id)
    def pick(reader):
        near = [n for n in notes if n[1].id in set(following.get(reader.id, []))]
        return rng.choice(near if near and rng.random() < 0.8 else notes)
    replies, likes, reposts = [], [], []
    jobs = []
    for reader in crew:
        activity = rng.lognormvariate(1.2, 0.8)
        for _ in range(int(activity * 8)):
            nid, author = pick(reader)
            if author is reader:
                continue
            kind = rng.random()
            if kind < 0.68:
                jobs.append((react, reader, 'POST', f'/likes/{nid}/'))
                likes.append(nid)
            elif kind < 0.9:
                def reply(r, n, text):
                    made = react(r, 'POST', f'/notes/{n}/replies', {'content': text})
                    if made:
                        replies.append((int(made['id']), n))
                jobs.append((reply, reader, nid, rng.choice(REPLIES)))
            else:
                jobs.append((react, reader, 'POST', f'/reposts/{nid}/'))
                reposts.append(nid)
    together(jobs)
    print(f'{len(likes)} likes, {len(replies)} replies, {len(reposts)} reposts.')

    # Dated over three years, an answer soon after what it answers.
    sql, dated = [], {}
    for nid, _p in notes:
        when = now - timedelta(minutes=rng.randint(60, 3 * 365 * 24 * 60))
        dated[nid] = when
        sql.append(f'UPDATE nodes SET created_at = {q(when)}, updated_at = {q(when)} WHERE id = {nid};\n')
    for rid, nid in replies:
        when = min(dated[nid] + timedelta(minutes=rng.randint(5, 4000)), now)
        sql.append(f'UPDATE nodes SET created_at = {q(when)}, updated_at = {q(when)} WHERE id = {rid};\n')
    for i in range(0, len(sql), 400):
        mysql(''.join(sql[i:i + 400]))
    ids = ','.join(str(p.id) for p in crew)
    # Most follows long ago, some lately, as a site's are.
    mysql(f"UPDATE edges SET created_at = NOW() - INTERVAL IF(RAND() < 0.1, FLOOR(RAND() * 30), 30 + FLOOR(RAND() * 900)) DAY "
          f"WHERE type = {q(FOLLOW)} AND node_b_id IN ({ids})")
    report(crew, len(notes))


def report(crew, posts):
    ids = ','.join(str(p.id) for p in crew)
    print('How even it is:')
    for label, sql in (
        ('Posts', f"SELECT created_by, COUNT(*) FROM nodes WHERE type = {q(NOTE)} AND parent_id IS NULL AND deleted_at IS NULL AND purged_at IS NULL GROUP BY 1"),
        ('Followers', f"SELECT follower.node_b_id, COUNT(*) FROM edges follower WHERE follower.type = {q(FOLLOW)} AND follower.node_b_id IN (SELECT id FROM nodes WHERE type = {q(PERSON)} AND purged_at IS NULL) GROUP BY 1"),
    ):
        counts = sorted((int(r[1]) for r in mysql(sql)), reverse=True)
        total = sum(counts) or 1
        top = counts[0] if counts else 0
        print(f'  {label}: {total} among {len(counts)}, median {statistics.median(counts) if counts else 0:g}, '
              f'the most {top} ({100 * top // total}%), the top ten {100 * sum(counts[:10]) // total}%')


sys.stdout.reconfigure(line_buffering=True)
main()
