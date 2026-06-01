# erg -- charte de design (phase Imagine, 2026-06-01)

> Document de specs issu d'une session d'exploration libre. Il fige une
> direction et un decoupage en chantiers ; il n'est PAS un plan de PR unique.
> Revu par 4 agents (auto-portance / coherence des artefacts / deploiement /
> alignement) ; les risques qu'ils ont leves sont integres ci-dessous.

## Glossaire (le document doit tenir sans la conversation d'origine)

- **`/merge`** : skill du harnais qui ferme le ticket lie et merge la PR de
  facon atomique. Indisponible hors harnais (p.ex. Claude Code web) -- d'ou le
  besoin d'un mecanisme d'autoclose qui n'en depend pas.
- **no-direct-push** : regle du harnais -- aucun commit direct sur `main`, tout
  passe par une merge request ; `main` est read-only. Un bot d'autoclose qui
  pousserait un commit de fermeture la violerait ; l'enforcement CII cote auteur
  ne la viole pas.
- **P2** : priorite 2 -- a livrer apres les chantiers `init`/`install`, non
  bloquant pour le decoupage 2->3->5->6. (Designe l'option d'autoclose retenue,
  cf. chantier 7.)
- **Modele conffile dpkg** : analogie Debian. On compare 3 etats -- *embarque*
  (ce que le binaire contient) / *sur-disque* / *estampille de reference* (ce
  qu'un `init` precedent a ecrit) -- pour distinguer un upgrade propre d'une
  edition locale. Aucune fusion 3-way reelle n'est requise : juste la decision
  skip / ecraser / preserver du tableau en chantier 4.
- **discover / act / land** : decoupage d'une fermeture de ticket. *discover* =
  trouver quel ticket est concerne (forge-specifique) ; *act* = `erg close` +
  `erg archive` (erg core) ; *land* = faire atterrir le changement sur `main`
  (cote auteur, dans sa PR).
- **binaire voyageur vs binaire systeme** (vocabulaire README "Binary policy",
  conserve) : le *voyageur* est `tickets/erg` commite (linux-amd64, pour CI et
  agents) ; le *systeme* est la copie installee sur le PATH de l'utilisateur.

## Architecture cible -- trois couches

| Couche | Quoi | Forge-aware | Present ou |
|---|---|---|---|
| **erg core** (binaire Go) | validate/check/list/new/close/archive/init/**install**/spec/integration/update + hooks git | **non** -- offline, local | partout, y compris Claude web |
| **erg-github** (script commite dans le repo) | `install` (pose le check CI) + `verify` (discover forge -> pass/fail) | oui, par forge | accelerateur optionnel |
| **AGENTS.md** (instructions) | "lance `erg close`" | n/a | plancher universel |

> Note : dans erg core, **`install` est le seul verbe qui mute hors de
> `tickets/`** (`.git/hooks`, AGENTS.md racine), et uniquement derriere des flags
> explicites par defaut a off (`--hooks`, `--inject-agents`). Tous les autres
> verbes restent confines a `tickets/` et offline.

---

## Blockers transverses -- a regler AVANT ou DANS les chantiers concernes

Issus de la revue. Aucun chantier ne doit etre considere "fini" sans le sien.

1. **Dispatch `erg-github` non specifie** (alignement/decouvrabilite). `main.go`
   est un `switch` fixe avec `default: Unknown command: X` ; il n'y a aucun
   fall-through PATH facon git, et `--help` ne liste aucun helper `erg-*`.
   L'analogie git est invoquee sans la machinerie qui la rend vraie.
   **Decision (chantier 8) :** soit (a) implementer le dispatch git-style
   (commande inconnue `foo` -> chercher `erg-foo` sur le PATH et l'exec ;
   `printUsage` liste les helpers decouverts), soit (b) abandonner l'analogie
   git, documenter `erg-github` comme script separe appris via AGENTS.md/README,
   et exiger un message d'erreur actionnable partout ou erg le reference
   (`erg-github introuvable -- il est commite a tickets/erg-github`).
2. **`erg archive` en pre-commit corrompt le commit** (deploiement/alignement).
   `archive.go` fait `os.Rename` ; en pre-commit, le fichier deplace n'est pas
   re-stage -> le commit enregistre une suppression sans l'ajout correspondant.
   **Decision (chantier 7) :** autoarchive en **pre-PUSH**, pas pre-commit (le
   ticket clos est deja commite a ce stade) ; le hook imprime ce qu'il a deplace.
   Interdire explicitement `erg archive` en pre-commit.
3. **Le test "deploye == embarque" est infaisable tel quel** (coherence). La
   copie deployee `tickets/AGENTS.md` de git-erg DIVERGE deja de
   `src/go/assets/AGENTS.md` (SHA differents ; elle contient un em-dash UTF-8 et
   du vocabulaire git-erg-specifique qui ferait echouer le test ASCII-only de
   `src/go/`). C'est un artefact volontairement personnalise, pas un clone.
   **Decision (chantier 4d/9) :** ne PAS asserter l'identite octet sur la copie
   dogfood ; asserter l'invariant plus faible "un `erg init` dans un arbre propre
   reproduit les assets embarques", + ajouter une cible Makefile de regeneration,
   + reconcilier la divergence existante comme precondition.
4. **`§3` orpheline des fichiers et casse des references** (deploiement). Reduire
   les assets ecrits 4->2 laisse `tickets/spec-erg-v1.md` et
   `tickets/integration.md` orphelins sur disque, et plusieurs docs pointent
   encore vers eux (README etapes 3-4, `AGENTS.md:40`, `main.go` manualPreamble,
   `integration.md` uninstall, `pep-erg-v1.md:233`). **Decision (chantier 3) :**
   nettoyage d'orphelins garde par hash (ne supprimer que si le fichier matche un
   hash embarque connu ; jamais une divergence = possible donnee utilisateur) +
   balayage doc complet dans la meme PR.
5. **Checklist "ajouter une sous-commande" non mentionnee** (coherence). Chaque
   nouveau verbe (`install`, `spec`, `integration`) doit toucher les 5 endroits
   imposes par `CONTRIBUTING.md` (fichier de commande, registre `helptext.go`,
   dispatch `main.go`, `tests/test_<cmd>.sh`, `TEST_SUITES` du Makefile), passer
   `TestDispatchRegistrySync` et `_test-lint`, et **bumper le compte de sections
   code en dur `16` dans `tests/test_docs.sh` (deux occurrences)**. A ajouter aux
   chantiers 2, 3, 7.

---

## Chantiers (rationale portable + derisque + surfaces completes)

### 1. Binaire & architecture
- Garder `tickets/erg` **unique**, **pas de suffixe d'arch**. *Rationale :* le
  levier anti-confusion est le `.gitignore` opt-out (un seul binaire sur disque
  pour qui ne commite pas), pas un suffixe ; le voyageur reste linux-amd64 pour
  CI/agents.
- `erg version` **bruyant** : chemin + arch + role, en reutilisant le
  vocabulaire README **voyageur / systeme** (ne PAS introduire "bootstrap /
  installed" qui divergerait du README).
- Surfaces : `src/go/version.go` (sortie), `summaryVersion`/`helpVersion`,
  `tests/test_version.sh`, README "Binary policy".

### 2. Split du verbe `init` / `install`
- **`erg init`** = semantique `git init` : echafaude `tickets/`, idempotent,
  **offline, ne touche rien hors `tickets/`**, **100 % assets** (jamais le
  binaire).
- **`erg install`** = cablage hors-boite (hooks, ligne AGENTS.md racine),
  **opt-in**.
- **Garde-fou anti-fat-finger conserve** (`erg init` refuse si `tickets/erg`
  absent). *Rationale :* `erg init` dans un repo sans `tickets/erg` est presque
  toujours un mauvais cwd, pas une intention ; refuser tot evite d'echafauder un
  `tickets/` orphelin.
- **Transition (derisque) :** apres le split, `erg init` n'arme plus les hooks.
  Pour ne pas casser silencieusement la memoire musculaire, `erg init` imprime
  un indice quand il ne detecte aucun cablage : *"Next: erg install --hooks pour
  armer le pre-commit"*. Aucun changement de comportement silencieux.
- **`migrate` (derisque) :** `migrate.go` appelle `installAssets` avec le meme
  `initAssetPaths` -> la reduction 4->2 (chantier 3) se propage correctement.
  Mais `migrate` utilise `installAssets(root, false)` (force-overwrite) :
  declarer explicitement qu'il **reste en force-overwrite** (la migration est une
  etape revue) et est **exempt du prompt dpkg** du chantier 4.
- Surfaces : `src/go/init.go` (scinder), nouveau `src/go/install.go`, dispatch
  `main.go`, registre `helptext.go`, `tests/test_install.sh`, `Makefile`
  TEST_SUITES, `tests/test_docs.sh` (compte de sections), README + UX-PROCESS.md
  (sequence d'install).

### 3. Assets deballes -> commandes a la demande
- **`erg spec`** et **`erg integration`** *impriment* le contenu embarque ;
  `tickets/` n'ecrit plus que **`.ergrc` + `AGENTS.md`** (`spec-erg-v1.md` et
  `integration.md` ne sont plus deposes).
- **Nettoyage d'orphelins (derisque, blocker #4) :** a l'`init`/`install`, si
  `tickets/spec-erg-v1.md` ou `tickets/integration.md` existent ET matchent un
  hash embarque connu (donc poses par un erg anterieur, pas ecrits a la main),
  les supprimer en loggant *"removed orphaned asset (now: erg spec / erg
  integration)"*. **Jamais** supprimer un fichier qui diverge de tout embarque
  connu (donnee possible).
- **Balayage doc (derisque, blocker #4) :** dans la meme PR -- README etape 3
  ("init ecrit .ergrc + AGENTS.md ; `erg integration` pour le setup, `erg spec`
  pour le format"), etape 4 ("lance `erg integration`") ; `AGENTS.md:40` ("lance
  `erg spec`/`erg integration`", plus de chemin de fichier) ; `main.go`
  manualPreamble (pointe `erg spec`, plus `tickets/spec-erg-v1.md`) ; `helpInit`
  (deux fichiers, plus "four files") ; `integration.md` uninstall (liste a 2
  fichiers) ; `pep-erg-v1.md:233`.
- Surfaces : `src/go/init.go` (`initAssetPaths`), nouveaux `src/go/spec.go` +
  `src/go/integration.go`, `bootstrap_assets.go` (embed conserve), registre
  `helptext.go`, dispatch `main.go`, `tests/test_spec.sh` + `test_integration.sh`,
  Makefile TEST_SUITES, `tests/test_docs.sh` (compte), + le balayage doc ci-dessus.

### 4. Versionnage des assets & drift -- **SOUS-DIVISE** (le plus gros, le plus risque)

Deux axes distincts a garder : **format** (`%erg 0.1` / spec `v1`) vs
**provenance build** (rev + date, deja dans `version.go`).

- **4a -- Manifeste de provenance.** Fichier `tickets/.erg-assets` ecrit par
  `init`/`install`. *A specifier explicitement (manquant aujourd'hui) :* le
  format (champs : rev, date, sha256 par asset ; ordre ; algo de hash) et le
  statut **commite vs gitignore** (decider en coherence avec la politique
  `.gitignore` du chantier 1 ; recommandation : commite, c'est de l'etat durable
  leger). Ecrire le manifeste **ne doit pas** declencher un echec pre-commit
  "tickets/ dirty".
- **4b -- Compare 3-etats (dpkg) avec defaut sur au premier run.** Tableau de
  decision :
  - sur-disque == embarque -> inchange (skip)
  - sur-disque == estampille mais != embarque -> upgrade propre (ecraser)
  - sur-disque != estampille ET != embarque -> edition locale (preserver/prompt)
  - **manifeste ABSENT (1er run apres upgrade, derisque blocker) :** ne PAS
    traiter comme "diverge" par defaut. Comparer le sur-disque a un **historique
    des hashes embarques deja expedies** baked dans `bootstrap_assets.go`. Match
    un release connu -> upgrade propre silencieux (+ ecrire l'estampille). Match
    aucun -> edition locale (prompt). Le binaire reconnait ainsi sa propre sortie
    passee, offline.
  - **Sortie loud (derisque alignement) :** `installAssets` n'imprime
    aujourd'hui que des compteurs agreges. Tout ecrasement doit **nommer chaque
    fichier** refresh/clobber + un indice `git restore -- <path>` pour que la
    reversibilite soit reelle, pas theorique.
- **4c -- Drift report.** `erg check` ajoute un warning quand l'estampille/le
  sur-disque divergent de l'embarque du binaire courant. Mettre a jour `helpCheck`
  (qui enumere l'ensemble exact des warnings) et `tests/test_check.sh`. Dans
  `update.go` : apres un swap de binaire reussi, si les hashes embarques du
  nouveau binaire different du sur-disque/manifeste, imprimer *"assets d'une rev
  anterieure -- relance erg init"* (miroir de l'indice Status: migration
  existant). Sinon le drift n'apparait qu'au prochain `check`.
- **4d -- Garde-fou CI d'auto-coherence (reframe, blocker #3).** NE PAS asserter
  l'identite octet sur la copie dogfood divergente. Asserter : (i) un `erg init`
  dans un arbre propre reproduit les 2 assets retenus ; et/ou (ii) les 2 assets
  retenus (`.ergrc`, `AGENTS.md`) existent et matchent l'embarque. Ajouter une
  **cible Makefile de regeneration** qui produit la copie deployee depuis
  `src/go/assets/` (sinon "genere" reste aspirationnel). Reconcilier la
  divergence actuelle de git-erg comme **precondition** (decider si git-erg
  arrete de personnaliser sa copie, ou documente qu'il utilise les defauts).
  Note : `tickets/` de git-erg ne suit aujourd'hui ni `.ergrc` ni
  `integration.md` -> le test ne couvre que les 2 assets reellement presents.

### 5. Mecanique d'`init`
- **`erg init -n` / `--dry-run`** : apercu (creerait/rafraichirait/sauterait/
  inchange). *Derisque :* `cmdInit` rejette aujourd'hui tout argument en `-`
  ("unknown flag") et `tests/test_init.sh` l'asserte -> reecrire le parse + le
  test.
- **`erg check` n'est PAS un alias de `init`/`install --dry-run`** (question
  ouverte du tableau Imagine : "? alias install dry run"). *Decide : non.* Deux
  verifications distinctes -- `--dry-run` previsualise l'install d'assets ;
  `check` verifie la sante du *corpus* de tickets. Les fusionner surchargerait
  `check`. Le versionnage ordonne (plus ancien/recent) du binaire reste a
  `update`, pas a init.
- **`erg init --force`** : exposer `installAssets(root, false)` (deja interne).
- **Codes de sortie (derisque) :** si on adopte 0/1/2, documenter un **petit jeu
  stable** sur une surface partagee (preambule manuel `main.go` ou spec) couvrant
  `check` ET `init` : `0`=ok, `1`=erreur dure, `2`=editions locales sautees.
  `helpCheck` documente deja "0 pass / 1 violation" -> aligner pour que `1` ne
  signifie pas deux choses. Les scripts existants `exit != 0 == fail` restent
  corrects (`2` reste un echec-ish).
- **Chainage `erg check` :** `init` lance un `check` read-only et imprime les
  warnings corpus, **mais son code de sortie reflete init**, pas les warnings.
  Documenter ce comportement dans `helpInit` (sinon surprise : warnings affiches
  mais exit 0).
- Surfaces : `src/go/init.go`, `helpInit`, `tests/test_init.sh`.

### 6. Frontiere de propriete (erg possede `tickets/` uniquement)
- **Ne mute hors-frontiere que sur flag explicite.**
- `--inject-agents` : suggere par defaut (n'edite pas), edition idempotente
  (grep du marqueur). *Derisque (consentement) :* si AGENTS.md racine est absent,
  **ne PAS le creer** sans un second opt-in explicite ; s'il est cree, imprimer
  le chemin et noter que le fichier est desormais user-owned. Documenter le
  retrait du bloc-marqueur (uninstall).
- `--hooks` : defaut off, marqueurs sentinelles, **jamais clobber** un pre-commit
  existant. *Derisque (migration marqueurs, blocker) :* la copie deployee utilise
  deja `# --- git-erg: begin/end managed block ---`. `erg install --hooks` doit
  **reconnaitre les deux** (legacy ET nouveau `# >>> erg managed >>>`) et
  remplacer en place, sinon il appendrait un SECOND bloc (validate/check
  dupliques). Aligner le texte de hook dans `integration.md` sur les memes
  marqueurs, meme PR. Ajouter un test "upgrade depuis marqueur legacy".
- Le pre-commit pose : validate + check + rejet `tickets/erg` hors main. (PAS
  archive -- cf. chantier 7, blocker #2.)
- Surfaces : `src/go/install.go`, `src/go/assets/integration.md`,
  `tests/test_install.sh`.

### 7. Autoclose / autoarchive -- decomposition asymetrique (P2)
- **Trailer git `Closes: 0042` : REJETE** (hors-distribution + redondant avec
  l'instruction deja donnee).
- **autoarchive -> mecanisable, zero convention** : `erg archive` en hook
  **pre-PUSH** (PAS pre-commit, blocker #2 ; le ticket clos est deja commite). Le
  header `Closed:` est le signal ; `check.go:folderClosure` le detecte deja ;
  `archive.go` balaye deja. Le hook imprime ce qu'il a deplace.
- **autoclose -> P2 "enforce", PAS d'autoclose-bot.** *Rationale :* l'exigence
  initiale etait un mecanisme *fiable* ne dependant pas de `/merge` ; un check CI
  requis pre-merge le fournit sans bot (respecte no-direct-push).
  **`erg-github verify`** = check requis pre-merge qui ECHOUE si la PR reference
  un ticket non-ferme : *"Please close ticket 0042 in this PR"*. Le close embarque
  dans la PR, fait par l'auteur (erg core + hook archive). Zero bot, zero acteur
  exempte. (cf. decoupage discover/act/land, chantier 8 : erg-github ne fait que
  *discover* ; le *close* est erg core ; le *land* est l'auteur.)
  - **Echappatoire (derisque alignement) :** le check passe si la PR ne reference
    aucun ticket (merge d'urgence non lie) ; documenter ce comportement (ou un
    label admin-override) pour ne pas wedger une PR legitime.
  - **Plancher merge local (pas de PR) :** hook pre-push (discover heuristique :
    nom de branche / refs) ou warning `erg check`.
- **Detection dans `erg check` gardee** : clos-pas-range (deja la). "Merge mais
  ouvert" sans lien fiable = assume non detecte.
- Surfaces : `erg-github` (script), `src/go/check.go`, hooks dans
  `src/go/install.go`.

### 8. `erg-github` -- la couche forge
- **Elision** : `erg-github` (pas `erg-forge-github`).
- **Deux verbes** : `install` (ecrit le YAML d'Actions, wrapper mince qui appelle
  `verify`, + notes de config) ; `verify` (discover forge via `gh` -> pass/fail ;
  pas de close/archive = erg core ; pas de landing = l'auteur).
- **Dispatch (blocker #1) :** trancher (a) dispatch git-style dans `main.go` +
  listing des helpers dans `--help`, ou (b) script separe documente + erreur
  actionnable. Nommer le mecanisme, pas seulement l'analogie.
- **Distribution & portabilite (derisque) :** commite dans le repo (voyage avec
  le clone, executable meme sans harnais). *A specifier :* interpreteur (POSIX
  `sh` de preference a bash ; si Python, `python3` + plancher de version) ;
  **degradation gracieuse** si `gh` absent/non-authentifie (`verify` sort avec un
  message clair et un chemin non-bloquant pour les adopteurs qui n'utilisent pas
  la forge) ; le wrapper YAML garde sur la forge (ne tourne que sur GitHub).
  **erg core ne depend jamais d'erg-github** (contrat offline preserve).
- **Decouvrabilite (derisque) :** pointeur README (ce qu'est erg-github, qu'il
  est commite et forge-aware) + une phrase dans AGENTS.md/integration.md ;
  decider si la regle pre-commit "rejet binaire hors main" s'applique a
  `erg-github` (c'est un 2e executable commite).

### 9. Self-hosting & doublon AGENTS.md
- **Garder l'embed** (contrat offline ; le binaire n'a aucun code HTTP --
  `update.go` fait `git fetch`).
- Copie deployee de git-erg -> **artefact genere** + cible Makefile de regen +
  test reframe (cf. 4d, blocker #3). `init`/`install` garde sa fonction.

---

## Decoupage en tickets (derisque)

1. `version` voyageur/systeme + doc opt-out binaire. *(§1)*
2. Split `init`/`install` + garde-fou conserve + indice de transition + exemption
   `migrate`. *(§2)*
3. `erg spec`/`erg integration` print-on-demand + **nettoyage orphelins** +
   **balayage doc** + checklist sous-commande. *(§3, blockers #4 #5)*
4. **Sous-divise :**
   - **4a** manifeste `.erg-assets` (format + statut commit) ;
   - **4b** compare 3-etats dpkg + **defaut sur sans-manifeste** (historique de
     hashes baked) + **sortie loud** ;
   - **4c** drift dans `check` + indice post-`update` ;
   - **4d** garde-fou CI reframe + cible Makefile regen + reconcilier la
     divergence dogfood. *(§4, §9, blocker #3)*
5. `init -n`/`--force` + reecriture parse de flags + chainage check documente +
   table de codes de sortie. *(§5)*
6. `erg install --hooks` (marqueurs + **migration legacy**) + `--inject-agents`
   (consentement creation). *(§6, blocker marqueurs)*
7. `erg-github install` + `verify` (P2 enforce) : **dispatch tranche** +
   **portabilite/degradation gracieuse** + autoarchive en **pre-PUSH** +
   echappatoire. *(§7, §8, blockers #1 #2)*

**Ordre suggere :** 2 -> 3 -> 5 (autour d'`init`), puis 6 -> 7 (install/forge),
puis 4a->4b->4c->4d (versionnage, transverse), 1 (cosmetique) a tout moment.

## Verification (par chantier)
- Tests Go/shell : `tests/test_<cmd>.sh` (note : `tests/test_init.sh` EXISTE
  deja, contrairement a une note anterieure), `make check`.
- Chaque nouvelle commande : `TestDispatchRegistrySync`, `_test-lint`, compte de
  sections `test_docs.sh`.
- 4d : le garde-fou doit echouer si on desynchronise les 2 assets retenus ; la
  cible regen doit reproduire la copie deployee.
- 7 : `erg-github verify` teste en echec (PR sans close) ET succes ; degradation
  testee (`gh` absent) ; hook pre-push archive end-to-end.
- ASCII-only sous `src/go/` (test d'encodage CI) ; ce document `docs/` peut
  rester en UTF-8.
