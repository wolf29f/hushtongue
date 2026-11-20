# hushtongue v2 — spec

Application web self-hosted remplaçant les deux apps TUI : un serveur Go qui sert une API JSON et le front React, sur Postgres, en un seul déploiement Docker.

## Objectifs

- Mêmes fonctionnalités que la v1 (voir « Reprise de la v1 »).
- Authentification : un compte MJ (identifiants en variables d'environnement), des comptes joueurs connectés par lien magique transmis par le MJ.
- Textes mémorisés pour y revenir plus tard, quand le dictionnaire a grandi.
- Note privée optionnelle sur une traduction et sur un texte mémorisé.
- Interface web, utilisable aussi sur téléphone à la table de jeu.

Hors périmètre : interface en anglais, partage de textes entre utilisateurs (les textes en jeu sont partagés autrement), vue du MJ sur les dictionnaires des joueurs, TUI (abandonnée).

## Séquencement

1. `v1.0.0` : la v1 actuelle.
2. `v1.1.0` : export du dictionnaire au format défini plus bas (« exporter », pas « explorer », dans ta réponse au point 4). Chaque utilisateur v1 exporte son dictionnaire avant de passer en v2.
3. `v2.0.0` : la v2 importe le fichier d'export. La TUI et SQLite sont retirés du dépôt dès le début du travail v2 (étape 2.1), la branche `v1` gardant la v1.

## Utilisateurs et rôles

- Deux rôles : `gm` et `player`. Le rôle remplace le build tag `gm` : les fonctions réservées au MJ (types de mots, génération) sont contrôlées par le serveur selon le rôle de la session, et masquées dans le front.
- Un seul MJ, défini par `HUSHTONGUE_GM_USERNAME` et `HUSHTONGUE_GM_PASSWORD`. Il est identifié par son rôle, une seule ligne `gm` dans `users` : elle est créée au démarrage si elle n'existe pas, et son nom est mis à jour depuis `HUSHTONGUE_GM_USERNAME`, qui ne sert qu'au login et à l'affichage. Changer ce nom garde donc le même compte et le même dictionnaire ; le mot de passe n'est pas stocké, il est comparé à la variable d'environnement en temps constant. Un hash en base n'apporterait rien tant que le mot de passe en clair reste la source dans la configuration ; le `.env` est protégé par ses droits (`chmod 600`).
- Les joueurs sont créés par le MJ (un nom affiché, unique), sans mot de passe.
- Chaque utilisateur, MJ compris, a son propre dictionnaire, ses textes et ses notes. Personne ne voit les données d'un autre, MJ compris.
- Direction par défaut inchangée : la langue que l'utilisateur connaît le moins (source pour le MJ, conlang pour les joueurs).

## Authentification

- Session : cookie `HttpOnly`, `SameSite=Lax`, contenant un token aléatoire ; la base n'en stocke que le hash (`sessions`). Expiration glissante : 30 jours pour le MJ, 1 an pour un joueur. Les sessions expirées sont purgées au démarrage puis une fois par jour. Chaque requête hache le token du cookie en SHA-256 et cherche la session par ce hash, jointe à son utilisateur, en une requête sur la clé primaire ; pas de cache, pour que la déconnexion et la régénération d'un lien prennent effet immédiatement. L'expiration n'est repoussée qu'au plus une fois par jour, pour ne pas écrire en base à chaque requête.
- Le cookie est `Secure` sauf quand `HUSHTONGUE_PUBLIC_URL` est en `http://` : Safari refuse un cookie `Secure` sur `http://localhost`, ce qui casserait le dev.
- Login MJ : formulaire nom + mot de passe, limité à quelques tentatives par minute et par IP. Derrière Traefik, l'IP du client est la dernière entrée de `X-Forwarded-For`, celle ajoutée par Traefik, que le client ne peut pas falsifier ; le conteneur ne doit donc pas être exposé directement.
- Lien magique : réutilisable, valable jusqu'à sa régénération. Il pointe vers la page du front `/login/{token}`, qui appelle `POST /api/login/link` avec le token ; le serveur crée la session et pose le cookie, puis le front retire le token de l'URL (`history.replaceState`). Un `GET` qui créerait la session serait déclenché par les aperçus de liens des messageries (Discord, WhatsApp, Signal…).
- Le token du lien est stocké haché : le lien n'est affiché qu'une fois, à sa création ou régénération. Le MJ qui l'a perdu le régénère.
- Régénérer le lien d'un joueur invalide l'ancien lien et ferme toutes les sessions de ce joueur. Supprimer un joueur supprime son dictionnaire, ses textes et ses sessions, après confirmation.
- Protection CSRF, sur toute requête qui modifie (méthode autre que `GET`, `HEAD` et `OPTIONS`) :
  - `SameSite=Lax` : le navigateur ne joint pas le cookie à une requête venant d'un autre site.
  - `http.CrossOriginProtection` de la bibliothèque standard : refuse (`403`) une requête dont `Sec-Fetch-Site` n'est pas `same-origin`, ou, sans cet en-tête, dont l'`Origin` diffère du `Host`.
  - `Content-Type: application/json` exigé (`415`), même sans corps : un formulaire d'un autre site ne peut pas l'envoyer sans preflight CORS.
- Déconnexion : supprime la session courante.

## Modèle de données (Postgres)

- `users` : `id`, `name`, `role` (`gm`/`player`), `login_token_hash` (null pour le MJ), `created_at`.
- `sessions` : `token_hash` (clé), `user_id`, `expires_at`, `created_at`.
- `words` : comme en v1 plus `user_id`. Unicité sur `(user_id, lang, normalized, kind)`, et sur `(user_id, id)` pour les clés étrangères composites.
- `translations` : comme en v1 plus `user_id` et `note` (texte, nullable). Clés étrangères composites `(user_id, source_word_id)` et `(user_id, con_word_id)` vers `words (user_id, id)` : la base interdit un lien entre les mots de deux utilisateurs. La colonne `source` (`manual`/`generated`), jamais lue en v1, est renommée `origin` et affichée dans la fiche du mot.
- `texts` : `id`, `user_id`, `lang` (langue du texte saisi), `content`, `note` (nullable), `created_at`, `updated_at`.
- Suppression d'un utilisateur : `ON DELETE CASCADE` sur toutes les tables.
- Migrations avec goose (`pressly/goose`, API `Provider`, version épinglée) : fichiers SQL embarqués dans le binaire, appliqués au démarrage sous le verrou consultatif Postgres de goose (`lock.NewPostgresSessionLocker`), pour que des redémarrages simultanés ne rejouent pas une migration. goose passe par `database/sql`, branché sur le pool `pgx` avec `stdlib.OpenDBFromPool`. Migrations « up » uniquement : une erreur se corrige par une nouvelle migration. `goose status` permet d'inspecter la base en production.
- `pg_trgm` : non utilisé. Le lexique est chargé en mémoire côté Go et fait déjà la complétion par trigrammes ; avec quelques milliers de mots par utilisateur, rien ne justifie une extension. On le reconsidérera si une recherche plein texte dans les textes mémorisés devient nécessaire. Aucune extension n'est donc requise sur l'instance existante.

## Isolation entre utilisateurs

- L'utilisateur est un paramètre explicite de chaque appel, après le `context.Context` : le store Postgres (`Word(ctx, userID, id)`, `DeleteTranslation(ctx, userID, id)`, `MergeWords(ctx, userID, fromID, intoID)`…), `lexicon.Load(ctx, d, userID, lang)`, `composer.Propose(ctx, userID, wordID)`, etc. Le serveur le prend dans la session.
- Toute requête SQL filtre par `user_id`, y compris celles par id : un id d'un autre utilisateur est traité comme inexistant (`storage.ErrNotFound`, comme un id qui n'existe pas).
- Tests requis : pour chaque méthode prenant un id, l'utilisateur A ne peut ni lire ni modifier un mot, une traduction ou un texte de B.

## Textes mémorisés

- Depuis la page de traduction, « Mémoriser le texte » enregistre le texte brut, sa langue et une note optionnelle. Les traductions choisies ne sont pas gardées.
- À la réouverture, le texte est relu avec le dictionnaire actuel : couleurs et traductions sont celles du moment, choix par défaut compris.
- La page « Textes » liste les textes de l'utilisateur (début du texte, note, date), du plus récent au plus ancien. Ouvrir un texte le charge dans la page de traduction ; « Enregistrer » met à jour le texte ouvert, « Mémoriser comme nouveau » en crée un autre.
- Suppression d'un texte après confirmation.

## Notes

- Une note privée optionnelle sur un lien de traduction, éditable depuis la fiche du mot et visible dans la page de traduction (survol ou appui long sur la traduction choisie).
- Une note optionnelle sur un texte mémorisé, éditable depuis la page de traduction et affichée dans la liste des textes.

## Mots et symboles

- Le texte saisi est découpé côté serveur en jetons : des mots et des symboles. Chaque signe de ponctuation ou symbole (`?`, `!`, `,`, `«`, `…`…) est un jeton à part : `viens?` donne `viens` et `?`.
- L'apostrophe, le trait d'union et l'underscore restent dans le mot quand ils sont entre deux lettres ou chiffres : `aujourd'hui`, `peut-être`, `mot_composé` sont un seul jeton.
- Les espaces d'origine sont gardés entre les jetons, pour reconstruire le texte traduit avec la même mise en forme.
- Un symbole s'ajoute au dictionnaire comme un mot (`?` en langue source, traduit par un mot de la conlang qui marque l'interrogation) et est cherché tel quel, sans retrait de la ponctuation. Le MJ peut en générer la traduction comme pour un mot.
- Un symbole absent du dictionnaire est recopié tel quel et affiché en neutre, pas en rouge. « Mémoriser les nouveaux mots » ignore les symboles ; on en ajoute un volontairement, depuis le dictionnaire ou avec le `+` de la page de traduction.
- Un symbole traduit par un mot est séparé par des espaces comme un mot (`tu viens?` donne `kaso na`). Un mot traduit par un symbole est collé au jeton précédent (`kaso na` donne `viens?`). Un symbole non traduit garde son espacement d'origine. L'espace française avant `?`, `!`, `:` et `;` pourra devenir une option plus tard.

## Reprise de la v1

Toutes les fonctionnalités de la v1 sont reprises :

- Traduction : choix de direction en gardant la saisie, coloration des mots (vert/orange/rouge), complétion (5 suggestions et `…`), choix d'une traduction par mot, ajout (`+`) et suppression d'un lien depuis la page, mémorisation des mots inconnus, copie du texte traduit, collage d'un texte.
- Dictionnaire : liste par langue avec filtre, ajout d'un mot, fiche du mot (traductions, ajout par mot existant ou nouveau, suppression d'un lien, renommage avec proposition de fusion, suppression du mot).
- MJ uniquement : changement de type de mot (avec suppression des liens après confirmation), génération de traduction avec décompositions, propositions classées et marquage des homonymes.
- La saisie mot par mot de la TUI devient une saisie libre : le texte est découpé en jetons côté serveur (voir « Mots et symboles »), normalisés avec le même `Normalize` qu'en v1. `Clean`, qui retirait la ponctuation autour des mots, disparaît avec le découpage en jetons.
- Le débordement en hauteur de la page de traduction, limitation connue de la v1, disparaît avec une page web qui défile.

## API

JSON sous `/api`, authentifiée par le cookie de session ; `401` sans session, `403` pour une fonction MJ appelée par un joueur, `404` pour un id d'un autre utilisateur.

- Erreurs : `{"error": "<code stable>", "message": "<texte en français>"}`. Le front affiche le message et réagit au code pour les cas particuliers (homonyme, fusion…).
- Limites : timeouts de lecture, d'écriture et d'inactivité sur le serveur HTTP ; corps de requête limité à 10 Mo sur toutes les routes, la taille dont l'import a besoin.


- Auth : `POST /api/login`, `POST /api/logout`, `GET /api/me` (nom, rôle), `POST /api/login/link` (token du lien magique).
- Réglages : `GET /api/settings`, sans session pour que la page de connexion puisse les afficher : libellés des langues, lus dans `HUSHTONGUE_SOURCE_LANG_LABEL` et `HUSHTONGUE_CON_LANG_LABEL`.
- Joueurs (MJ) : `GET/POST /api/players`, `POST /api/players/{id}/link` (régénère, renvoie le lien une fois), `DELETE /api/players/{id}`.
- Mots : `GET /api/words?lang=`, `POST /api/words`, `GET/PATCH/DELETE /api/words/{id}`, `POST /api/words/{id}/merge`, `PUT /api/words/{id}/kind` (MJ).
- Traductions : `POST /api/words/{id}/translations`, `PATCH /api/translations/{id}` (note), `DELETE /api/translations/{id}`.
- Génération (MJ) : `GET /api/words/{id}/proposals`, `POST /api/words/{id}/proposals/apply`.
- Traduction : `QUERY /api/translate` (langue, texte → mots découpés avec statut et traductions), `GET /api/complete?lang=&q=` (suggestions). `QUERY` (RFC 10008) est la méthode sûre et idempotente avec un corps ; `POST` en repli si le reverse proxy la rejette (à vérifier avec `curl -X QUERY` à travers Traefik).
- Textes : `GET/POST /api/texts`, `GET/PUT/DELETE /api/texts/{id}`.
- Import : `POST /api/import`, corps `{"file": "<fichier d'export en base64>"}`. Le fichier reste un document opaque, séparé de la requête qui le transporte : le serveur le décode puis le lit avec sa propre validation (`format`, `version`), indépendamment du schéma de la requête, et l'enveloppe peut recevoir des options plus tard. Le front l'envoie en JSON, pas en multipart, comme toute requête qui modifie (voir CSRF).

Le découpage en jetons, `Normalize`, la recherche et la complétion restent uniquement côté Go : le front n'en a pas de copie, pour qu'ils ne divergent pas. Le lexique de l'utilisateur est mis en cache en mémoire côté serveur et invalidé à chaque écriture de cet utilisateur.

## Export v1.1.0 et import v2

- Les joueurs ne connaissent pas forcément le terminal : l'export se fait depuis un menu de la TUI, sans ligne de commande.
- Une entrée « Exporter le dictionnaire » du menu d'accueil ouvre une page avec un champ de chemin, prérempli avec `hushtongue-export-<date>.json` dans le dossier Bureau du système (`xdg.UserDirs.Desktop` : `~/Desktop` sous macOS et Linux, `%USERPROFILE%\Desktop` sous Windows). Le chemin est modifiable ; `~` y est accepté.
- Un fichier existant n'est écrasé qu'après confirmation ; un dossier manquant est une erreur, pas créé. En cas de succès, la page affiche le chemin complet du fichier écrit.
- Format, versionné car c'est le contrat entre v1 et v2 :

```json
{
  "format": "hushtongue-export",
  "version": 1,
  "words": [{ "id": 1, "lang": "source", "text": "mer", "kind": "root" }],
  "translations": [{ "source": 1, "con": 2, "origin": "generated" }]
}
```

- Les `id` ne servent qu'à relier les traductions aux mots dans le fichier.
- L'import v2 se fait dans le dictionnaire de l'utilisateur connecté, en upsert sur `(lang, normalized, kind)` et sur le couple de mots pour les traductions : réimporter le même fichier ne change rien, importer dans un dictionnaire non vide fusionne.
- Chaque utilisateur importe son propre fichier une fois connecté : le MJ le sien, l'autre joueur le sien via son lien magique.

## Front

- React, TypeScript 7 (le compilateur natif), Vite. React Router en data mode pour les pages et les données : chaque route a un loader (lecture avant l'affichage) et, si elle a un formulaire, une action ; après une action, les loaders de la page sont relancés, ce qui recharge les données sans cache à invalider. La table des routes, avec ses loaders et ses actions, est dans `src/routes.tsx`, comme celle du serveur dans `server.go`. Les réglages de l'instance (`GET /api/settings`) sont lus une seule fois au démarrage par un `SettingsProvider` qui englobe le routeur, et chaque composant les lit avec `useSettings`. `fetch` natif pour les appels à l'API. Pas de bibliothèque de cache (TanStack Query, essayé à l'étape 3.1, a été retiré à la 3.2) : le front doit rester simple à relire, et un petit helper écrit à la main est préféré à une bibliothèque au comportement caché. pnpm comme gestionnaire de paquets, avec sa version épinglée dans `packageManager`, des versions de dépendances exactes et `pnpm install --frozen-lockfile` en CI ; son délai par défaut avant d'installer une version tout juste publiée est gardé, contre les paquets compromis. Tailwind CSS 4 pour les styles, pas de bibliothèque de composants.
- Pages : Connexion, Traduction (accueil), Dictionnaire, Fiche du mot, Génération (MJ), Textes, Joueurs (MJ).
- Raccourcis clavier de la v1 gardés quand ils ont un sens sur le web (navigation dans les suggestions, choix des traductions), chaque action restant faisable à la souris et au doigt.
- Interface en français.
- Outils : `tsc`, oxlint et oxfmt (avec le tri des classes Tailwind), l'équivalent de golangci-lint côté web ; ESLint et Prettier sont écartés car typescript-eslint ne prend pas en charge TypeScript 7, et oxlint et oxfmt sont les outils du gabarit officiel de Vite. oxfmt étant encore en bêta, sa version est épinglée et ses mises à jour se font dans un commit à part, qui peut reformater beaucoup de fichiers ; Vitest pour la logique pure du front, pas de tests de bout en bout pour l'instant.

## Dépôt et déploiement

- Un seul dépôt, un seul module Go à la racine, l'app React autonome dans `web/` :

```
hushtongue/
├── go.mod                  # module Go unique, avec `ignore ./web/node_modules`
├── Dockerfile
├── compose.yaml            # service complet : Postgres et hushtongue buildé
├── compose.dev.yaml        # dev : Postgres seul
├── Makefile                # raccourcis de dev, de contrôle et de build
├── cmd/server/main.go      # lit la config, ouvre Postgres, lance le serveur
├── internal/
│   ├── config/             # variables d'env (remplace le build tag gm)
│   ├── server/             # HTTP : table des routes (server.go)
│   │   ├── httpapi/        # commun : JSON, format d'erreur, utilisateur de la requête
│   │   ├── middleware/     # JSON (404/405, Content-Type), CrossOrigin, RequireSession, RequireGM
│   │   ├── auth/           # sessions, login, logout
│   │   ├── frontend/       # fichiers du front, index.html hors /api
│   │   └── players/, health/, settings/, words/…  # un paquet par fonctionnalité
│   └── services/           # métier : generator, lexicon, composer
│       └── storage/
│           ├── storage.go
│           └── postgres/   # remplace sqlite/, migrations embarquées
└── web/                    # app React (Vite, TypeScript)
    ├── package.json, vite.config.ts, tsconfig.json, index.html
    ├── src/
    │   ├── api/            # client et types de l'API, écrits à la main
    │   ├── pages/          # une page par écran
    │   ├── components/
    │   ├── routes.tsx      # table des routes, loaders et actions
    │   └── main.tsx
    ├── embed.go            # //go:build embed  → embarque dist/
    ├── noembed.go          # //go:build !embed → page « front non buildé »
    └── dist/               # sortie Vite, ignorée par git
```

- `internal/tui`, `storage/sqlite` et `.goreleaser.yaml` sont retirés.
- Le paquet Go `web` expose le front buildé comme `fs.FS` ; il est dans `web/` car `go:embed` ne peut pas remonter avec `..`. Seuls le Dockerfile et `make build` buildent avec `-tags embed` : sans le tag, `go build`, `go test` et golangci-lint marchent sans Node, et rien de vide n'est à committer dans `dist/`.
- `ignore ./web/node_modules` dans `go.mod` : certains paquets npm contiennent des fichiers `.go`, qui casseraient `go build ./...` et le lint.
- Le serveur sert les fichiers statiques et renvoie `index.html` pour toute route hors `/api` (paquet `internal/server/frontend`, qui reçoit le `fs.FS` de `web.Dist()` par `server.New`, et un `fstest.MapFS` en test). Les fichiers de `assets/`, nommés d'après le hash de leur contenu, sont mis en cache définitivement.
- En dev : `make db` lance Postgres seul (`compose.dev.yaml`), `make run` le serveur Go, `make web-dev` le serveur de dev de Vite. Vite transmet `/api` au serveur Go ; le cookie reste sur la même origine, sans CORS.
- Pas d'outillage monorepo (un seul paquet JS), pas de génération de types depuis Go pour l'instant.
- Serveur HTTP : bibliothèque standard (`net/http`, routage par motifs, `encoding/json/v2`, actif par défaut depuis Go 1.27), `pgx/v5` pour Postgres.
- Organisation de `internal/server` : un sous-paquet par fonctionnalité (`auth`, `players`, `health`, `settings`, puis `words`, `translations`, `generation`, `translate`, `texts`, `import`), plus `httpapi` pour ce qu'ils partagent (lecture et écriture JSON, format d'erreur, `PathID`, utilisateur de la requête) et `middleware` pour tous les middlewares (réponses 404/405 en JSON et contrôle du `Content-Type`, protection cross-origin, `RequireSession`, `RequireGM`).
  - Les middlewares ont la forme standard `func(http.Handler) http.Handler`, pour s'enchaîner librement. `RequireSession` met l'utilisateur dans le contexte de la requête (`httpapi.WithUser`) ; un handler le lit avec `httpapi.User(r.Context())`, qui panique sans utilisateur : lire l'utilisateur sur une route sans `RequireSession` est une erreur de câblage, couverte par les tests de `internal/server`.
  - Chaque fonctionnalité a une struct `Handlers` créée par `New`, dont les méthodes sont les handlers, sans variable globale. Ses dépendances sont des interfaces déclarées dans le paquet, réduites aux méthodes qu'il appelle (`players.Store`, `health.Pinger`…) : un handler ne voit que ce dont il a besoin, et ses tests passent de petits faux écrits à la main, sans Postgres.
  - `httpapi` n'importe aucune fonctionnalité ; une fonctionnalité peut en importer une autre pour un helper (`players` utilise `auth.NewToken` et `auth.HashToken`, `middleware` le cookie et les durées de session d'`auth`), sans cycle.
  - Une seule table des routes dans `server.go`, avec le middleware de chaque route (`middleware.RequireSession`, `middleware.RequireGM`). `server.New` passe le `*postgres.Store` à chaque `New`, ce qui vérifie à la compilation qu'il satisfait leurs interfaces.
  - Handlers fins : lire la requête, appeler le service, écrire la réponse ; la logique métier reste dans `internal/services`.
  - Les services renvoient des erreurs sentinelles (`storage.ErrNotFound`…), converties en code HTTP et `httpapi.Error` par une seule fonction, `httpapi.WriteServiceError`.
  - Tests en deux couches : chaque fonctionnalité sur ses faux ; `internal/server` sur Postgres à travers la table des routes, pour le câblage (routes réservées au MJ, CSRF, 404/405) et un parcours complet (login, création d'un joueur, connexion par lien, régénération, suppression).
- Dockerfile en trois étapes : build Node du front, build Go, image finale minimale (distroless) avec le seul binaire. Le HTTPS est assuré par le reverse proxy.
- Deux fichiers Compose, avec des noms de projet distincts pour ne pas partager la base : `compose.yaml` lance le service complet (Postgres et l'image buildée, URL publique `http://localhost:8080`), `compose.dev.yaml` Postgres seul, exposé sur `127.0.0.1`. Les identifiants de dev viennent d'un `.env` ignoré par git, que `make` copie depuis `.env.example` au premier lancement, et que lisent Compose et le Makefile.
- Un Makefile plutôt qu'un Taskfile : `make` est déjà installé partout, Task serait une dépendance de plus. Il reste une couche fine sur les commandes du README et de la CI, compatible avec le GNU Make 3.81 de macOS.
- Santé, sans authentification :
  - `GET /api/health/live` : `200` dès que le serveur répond, sans regarder la base. Un redémarrage n'y changerait rien quand Postgres est indisponible.
  - `GET /api/health/ready` : `200` si la base répond à un ping en moins de 2 s, `503` sinon. Sert au `depends_on: condition: service_healthy` de Docker Compose et au health check de Traefik.
  - L'image minimale n'a ni `curl` ni `wget` : le binaire a une sous-commande `hushtongue healthcheck` qui appelle `/api/health/ready` en local et sort en `0` ou `1`, utilisée par le `HEALTHCHECK` du Dockerfile.
- Release : le workflow GitHub publie l'image Docker à chaque tag `v2.*` au lieu des binaires.
- CI sur chaque push : `go test` (testcontainers tourne sur les runners GitHub) et golangci-lint, puis build et lint du front à partir de l'étape 3.1.
- Une seule instance du serveur : le lexique de chaque utilisateur est mis en cache en mémoire et la limite de tentatives du login est en mémoire, ni l'un ni l'autre n'étant partagé entre instances. Le README le précise, avec la piste pour passer à plusieurs instances : retirer le cache et recharger le lexique à chaque requête, sans passer à `pg_trgm`.
- Variables d'environnement :

| Variable | Requise | Défaut |
| --- | --- | --- |
| `DATABASE_URL` | oui | |
| `HUSHTONGUE_GM_PASSWORD` | oui | |
| `HUSHTONGUE_GM_USERNAME` | non | `mj` |
| `HUSHTONGUE_PUBLIC_URL` | non | déduite de la requête (`Host`, `X-Forwarded-Proto`) |
| `HUSHTONGUE_ADDR` | non | `:8080` |
| `HUSHTONGUE_SOURCE_LANG_LABEL` | non | `Français` |
| `HUSHTONGUE_CON_LANG_LABEL` | non | `conLang` |

## Sort de `IMPROVEMENTS.md`

- Interface en français et en anglais : reportée, hors v2.0.
- Statut des mots lisible sans les couleurs : repris dans le front (style en plus de la couleur).
- Emojis du menu d'accueil, presse-papiers, choix du fichier de base (`--db`) : sans objet en v2.
- Libellés des langues stockés en base : remplacés par des variables d'environnement, l'instance n'ayant qu'une conlang.
- Jonction préfixe-racine : inchangée, indépendante de la v2.
- Nettoyage : `translations.source` est affichée (voir modèle de données) ; le bloc de spec obsolète de `wordview.go` disparaît avec la TUI.

## Pistes pour plus tard

- Recherche dans les textes mémorisés par les mots qu'ils contiennent : recherche plein texte Postgres (`tsvector` et index GIN) ou `pg_trgm`, selon le besoin.
- Titre optionnel sur un texte mémorisé.
- Espace française avant `?`, `!`, `:` et `;` dans le texte traduit.

## Étapes

1. `v1.1.0` : export.
2. Socle serveur, une étape par commit testable :
   1. Nettoyage : retrait de la TUI, du build tag `gm` et de `.goreleaser.yaml`. Une branche `v1` part du tag `v1.1.0` pour d'éventuels correctifs.
   2. Config et serveur minimal : `internal/config` lit et valide les variables d'environnement (erreur claire au démarrage si une variable requise manque), `cmd/server` lance `net/http` avec arrêt propre sur SIGTERM, logs JSON sur la sortie standard, format d'erreur et limites HTTP, `GET /api/health/live`, sous-commande `healthcheck`, workflow de CI.
   3. Postgres : pool `pgx`, migrations goose avec verrou Postgres, `GET /api/health/ready`, schéma initial complet (`users`, `sessions`, `words`, `translations`, `texts`). Les tests lancent un Postgres jetable avec testcontainers : un conteneur par paquet (`TestMain`), chaque test dans son propre schéma. `go test -short` les ignore ; sans `-short`, Docker absent fait échouer les tests avec un message clair plutôt que de les ignorer, pour qu'un run complet ne passe jamais au vert sans les avoir exécutés.
   4. Storage Postgres : l'interface `storage.Storage` prend l'utilisateur en paramètre, implémentation Postgres portée depuis `storage/sqlite` (requêtes et tests), tests d'isolation entre utilisateurs. `storage/sqlite` est supprimé dans ce même commit : le passage de l'utilisateur en paramètre le casse, et le porter n'aurait aucun intérêt. `lexicon` et `translation` (renommé `composer`) passent l'utilisateur, leurs tests sont adaptés.
   5. Auth : sessions (création, expiration glissante, suppression, purge), login MJ avec limite de tentatives, middleware (`401`, `403`, contrôle du `Content-Type`), `GET /api/me`, `POST /api/logout`.
   6. Joueurs : création, liste, suppression, régénération du lien (qui ferme les sessions du joueur), `POST /api/login/link`.
3. Socle front :
   1. `web/` : Vite, React, TypeScript, React Router, TanStack Query, Tailwind, oxlint, oxfmt, Vitest ; `embed.go`/`noembed.go` ; proxy Vite vers le serveur ; `ignore ./web/node_modules` dans `go.mod` ; front ajouté à la CI.
   2. Pages Connexion et `/login/{token}`, mise en page avec le menu selon le rôle ; passage de React Router en data mode, qui remplace TanStack Query. Le menu liste déjà les pages de la spec, celles à venir affichant « Cette page arrive bientôt ».
   3. Page Joueurs (MJ).
4. Import v1 (API et bouton dans le front).
5. Dictionnaire et fiche du mot.
6. Traduction.
7. Génération (MJ).
8. Notes et textes mémorisés.
9. Workflow de release (image Docker), `v2.0.0`. Le Dockerfile, les fichiers Compose et le Makefile sont arrivés après l'étape 3.1.
