# CryptoBulle

**Chiffrer et déchiffrer du texte n'importe où dans Windows, avec deux raccourcis clavier.**

Vous sélectionnez du texte, vous faites `Ctrl+Alt+E` : le texte est remplacé par un
message chiffré. Vous sélectionnez ce message, vous faites `Ctrl+Alt+D` : une petite
**bulle** apparaît près de la souris et affiche le texte d'origine.

Un seul fichier `CryptoBulle.exe` de **2,91 Mo**. Rien à installer.

---

## Ce que ça fait

| Raccourci (modifiable) | Action |
| --- | --- |
| `Ctrl+Alt+E` | Chiffre la sélection et la colle à la place |
| `Ctrl+Alt+D` | Déchiffre la sélection et l'affiche dans une bulle |
| `Ctrl+Alt+M` | Allume ou éteint la **frappe masquée** |

- L'application vit dans la **zone de notification**, à côté de l'horloge. Clic droit :
  réglages, chiffrer, déchiffrer, quitter.
- La bulle se ferme seule après quelques secondes, sauf si la souris est dessus.
  Boutons **Copier** et **Fermer**, touche `Échap`, déplaçable à la souris.
- L'ancien contenu du presse-papiers est remis en place après l'opération.
- La fenêtre de réglages contient un **atelier** pour chiffrer ou déchiffrer à la main.
- Le chiffrement ne dit rien quand il réussit : le texte collé se voit déjà.

## Installation

Téléchargez `CryptoBulle.exe` et double-cliquez. C'est tout : pas d'installateur,
pas de Python, pas de bibliothèque à côté. Au premier lancement, la fenêtre de
réglages demande votre **phrase secrète**.

Pour construire vous-même, avec [Go](https://go.dev/dl/) installé :

```bat
build_windows.bat
```

Depuis un Mac ou un Linux, Go compile directement pour Windows :

```bash
./scripts/build.sh
```

L'action GitHub incluse fabrique aussi le `.exe` à chaque envoi de code :
onglet *Actions*, dernier build, artefact `CryptoBulle-windows`.

## Pourquoi Go

La première version était écrite en Python. Le passage à Go a supprimé la dernière
grosse pièce embarquée : l'interpréteur Python et sa bibliothèque graphique Tcl/Tk,
qui représentaient l'essentiel du poids.

| | Version Python | Version Go |
| --- | --- | --- |
| À livrer | un dossier avec l'interpréteur et Tcl/Tk | un seul `.exe` de 2,86 Mo |
| À installer sur le poste | rien, mais tout était embarqué | rien |
| Chiffrement d'une phrase | 230 µs | **2,0 µs** |
| Déchiffrement | 10 µs | **1,3 µs** |
| Compiler depuis un Mac | impossible | `./scripts/build.sh` |

Les durées ci-dessus sont mesurées sur la machine de test, pas sur un vrai poste
Windows. Le rapport entre les deux colonnes reste parlant : le même travail, cent
fois plus rapide.

**Ce que Go n'apporte pas.** L'idée d'un moteur en Go avec l'affichage resté en
Python aurait été le pire des deux mondes : il aurait fallu livrer l'exécutable Go
**et** l'environnement Python, plus la communication entre les deux. Le gain vient
justement d'avoir tout dans un seul programme.

## Ce qui fait que c'est rapide

1. La clé est **calculée à l'avance**, au démarrage, et gardée en mémoire. Le
   raccourci n'attend jamais scrypt (45 ms).
2. La bulle est une fenêtre **construite une fois puis réutilisée**.
3. La copie de la sélection surveille le **numéro de séquence** du presse-papiers,
   un compteur que Windows incrémente à chaque changement. On réagit dès que la
   copie est faite, sans attendre un délai fixe.
4. Au repos, le programme **dort vraiment** : `GetMessage` rend la main à Windows,
   qui ne réveille l'application que pour les deux raccourcis déclarés et les clics
   sur l'icône. Aucune boucle d'attente, aucun réveil périodique.

## La frappe masquée

`Ctrl+Alt+M` allume un mode où **rien de ce que vous tapez n'apparaît en clair**.
Chaque touche est interceptée avant d'atteindre le logiciel et remplacée à l'écran
par un caractère chiffré. Une lettre tapée donne un caractère affiché, en direct,
dans Word, un navigateur, une messagerie, n'importe où.

Quelqu'un qui regarde par-dessus votre épaule pendant que vous écrivez ne voit
jamais les vraies lettres. `Ctrl+Alt+D` sur le résultat le relit normalement,
c'est le même raccourci que pour les messages ordinaires.

- L'icône près de l'horloge passe à l'**ambre** tant que le mode est actif. C'est
  le seul indicateur, il n'y a pas de fenêtre qui s'ouvre : elle volerait le
  curseur au logiciel dans lequel vous écrivez.
- Le **retour arrière** efface le caractère et libère sa place, comme d'habitude.
- La touche **Entrée** passe telle quelle : un paragraphe entier reste un seul
  texte masqué.
- Si vous **changez de champ**, de fenêtre, ou déplacez le curseur, l'application
  repose son en-tête invisible toute seule. Chaque morceau reste donc lisible,
  où qu'il ait été écrit.
- Dans une **messagerie**, `Entrée` envoie le message et vide le champ :
  l'application le remarque et repose un en-tête pour le message suivant. Dans un
  traitement de texte, où `Entrée` saute simplement une ligne, elle continue sans
  rien ajouter. La différence se lit à la position du curseur.
- Les **emojis** et les caractères rares sont chiffrés eux aussi. Rien ne sort
  jamais en clair.
- La ligne ne porte **aucun marqueur** : c'est du charabia du premier au dernier
  caractère.
- **Échap** ou `Ctrl+Alt+M` éteint le mode.
- Les raccourcis du système continuent de fonctionner : `Ctrl+S`, `Alt+Tab` et
  compagnie passent sans être touchés. `AltGr` reste une touche d'écriture.

### Le bouton de test

Les réglages contiennent un bouton **Tester la frappe masquée**. Il vérifie la
chaîne complète tout seul, sans que vous ayez à taper quoi que ce soit :

1. le chiffrement lui-même, sans rien demander à Windows ;
2. l'installation de l'interception clavier ;
3. la frappe réelle, simulée dans le champ de l'atelier ;
4. la relecture du résultat.

Le rapport dit exactement où ça casse : rien n'est arrivé, le texte est passé en
clair, les caractères sont arrivés dans le désordre. Il affiche aussi votre
disposition clavier et le nombre de caractères attendu contre reçu. Le bouton
**Copier** met ce rapport dans le presse-papiers.

### Ce que ce mode ne fait pas

- Il **ne fonctionne pas** dans une fenêtre lancée en administrateur, ni dans
  certains jeux. Windows interdit à une application ordinaire d'intercepter les
  touches destinées à une fenêtre plus privilégiée.
- Il **protège moins bien** que le mode ordinaire. Chiffrer lettre par lettre est
  un chiffrement par flux, sans signature : la longueur du texte reste visible et
  une mauvaise phrase secrète donne du charabia au lieu d'une erreur claire. Pour
  un vrai secret, chiffrez le message entier avec `Ctrl+Alt+E`.
- Ne **déplacez pas le curseur** pendant la frappe. Les flèches et la souris ne
  sont pas suivies : le texte serait mélangé.
- Certains **antivirus** surveillent les programmes qui interceptent le clavier.
  Le vôtre peut demander confirmation la première fois.

Rien n'est enregistré. La touche est traduite puis oubliée, et l'interception
n'existe que pendant que le mode est allumé.

## Une interface qui ne fait pas datée

Une application Win32 a l'air vieille pour deux raisons précises, corrigées ici.

**Le manifeste.** Sans lui, Windows dessine les boutons et les champs de saisie
comme en 2000, gris et carrés. Le fichier `cmd/cryptobulle/app.manifest`, intégré
à l'exécutable, réclame les *common controls* version 6 : les contrôles prennent
alors l'apparence native de Windows 10 et 11.

**La densité de pixels.** Le même manifeste déclare l'application *per monitor v2*,
c'est-à-dire capable de se redimensionner elle-même écran par écran. Sans cela,
Windows agrandit l'image après coup et tout devient flou sur un écran haute
définition. Chaque fenêtre lit sa densité réelle, refait ses polices à la bonne
taille et se replace quand on la déplace vers un autre écran.

La bulle, elle, est entièrement dessinée :

- **Coins arrondis** confiés à Windows 11 (`DwmSetWindowAttribute`), donc lissés par
  le système. Sur Windows 10, ils restent droits, ce qui est le style de ce système.
- **Ombre portée** légère, la même que celle des menus.
- **Boutons aux bords lisses**, avec un effet au survol. GDI+ aurait pu les dessiner,
  mais ses fonctions attendent des nombres à virgule, que Go ne sait pas transmettre
  à une DLL sur processeur 64 bits. Ils sont donc peints à la main dans une petite
  image en mémoire : pour chaque pixel, on calcule quelle part est à l'intérieur de
  la forme, avec seize échantillons. C'est ce qui remplace l'anticrénelage.
- **Bande de couleur** à gauche : bleu pour une information, vert pour une réussite,
  rouge pour une erreur.
- Texte en Segoe UI, rendu par ClearType, le lissage de police de Windows.

Tout cela n'ajoute que 10 Ko à l'exécutable : le dessin utilise des fonctions déjà
présentes dans le système.

## Les deux formats, propres à l'application

Un message chiffré ressemble à ceci :

```
MC1~UZgbA1PclmY3ouGuGGioC9PSHYlNlj04crweWjpP19ZyE9QWdEOvpTIVjURKsQb5...
```

Trois couches :

1. **scrypt** transforme votre phrase secrète, plus une constante propre à
   l'application (le « poivre »), en une clé de 256 bits.
2. **AES-256-GCM** chiffre le texte avec cette clé, un **sel** et un **nonce** tirés
   au hasard.
3. Le résultat est encodé avec un **alphabet maison** : une permutation des 64
   caractères du base64, unique à CryptoBulle.

Un décodeur base64 en ligne ne renvoie donc que du bruit, et même le bon outil ne
sert à rien sans la bonne phrase secrète.

Le format n'a pas changé depuis la version Python. Un test automatique déchiffre un
message produit par l'ancienne version, pour garantir que vos anciens messages
restent lisibles.

La frappe masquée utilise un second format, qui doit produire un caractère dès
que vous en tapez un. AES en mode compteur fabrique une suite d'octets
imprévisible, et chaque octet décale la lettre tapée dans l'alphabet. Les dix
premiers caractères de la ligne portent le tirage aléatoire de la session et deux
caractères de contrôle. Ils ne se voient pas : ils ressemblent au reste.

L'alphabet couvre les lettres, les chiffres, l'espace, la ponctuation courante et
les dix accents les plus fréquents en français. Un emoji, une majuscule accentuée
ou tout autre caractère s'écrit en quatre caractères masqués : un signal
d'échappement, puis son numéro Unicode. C'est le seul endroit où un caractère tapé
n'en donne pas exactement un.

## Marqueur ici, rien là

Les deux modes ne se comportent pas pareil, et c'est voulu.

Un message chiffré avec `Ctrl+Alt+E` commence par **`MC1~`**. Vous le collez dans
une conversation, on voit tout de suite que c'est un message chiffré, et
l'application le retrouve dans votre sélection sans hésiter.

Une ligne tapée en **frappe masquée** ne porte **aucun marqueur**. Elle ne doit
ressembler à rien : personne ne doit deviner ce que vous êtes en train de faire en
regardant votre écran. Le texte n'utilise que des caractères ordinaires du
clavier, sans accent ni symbole exotique : ça ressemble à `fPyC@.WE^E`, pas à du
texte abîmé.

Sept caractères de plus s'ajoutent au début de chaque morceau de texte : quatre
pour le tirage aléatoire, trois pour le contrôle. Un morceau, c'est tout ce que
vous écrivez d'affilée au même endroit. Un paragraphe entier, retours à la ligne
compris, n'en porte donc qu'un seul. Changer de champ en pose un nouveau, ce qui
est exactement ce qui rend le nouveau champ lisible.

Ces sept caractères ne se voient pas, ils se lisent comme le reste. À la
relecture, l'application les reconnaît où qu'ils soient, même au milieu d'une
ligne.

Ces six caractères évitent le pire défaut possible : sans tirage aléatoire, deux
fois la même phrase donnerait exactement le même affichage, et quelqu'un qui
récolterait une vingtaine de vos lignes pourrait les casser sans connaître votre
phrase secrète.

Sans marqueur, comment l'application s'y retrouve ? Elle **essaie** de relire la
ligne. Les deux caractères de contrôle, calculés à partir de votre clé, lui disent
si le texte est bien le sien. Sur un texte ordinaire, la bulle répond simplement
qu'il n'y a rien à déchiffrer.

### Petit lexique

- **Chiffrer** : rendre un texte illisible sans la clé. (« Encrypter » est un anglicisme.)
- **Clé** : la suite de bits qui verrouille et déverrouille le message.
- **scrypt** : fonction qui fabrique une clé à partir d'un mot de passe, en étant
  volontairement lente et gourmande en mémoire, pour décourager les essais en masse.
- **Sel** : valeur aléatoire mélangée à la phrase secrète, pour que deux personnes
  ayant la même phrase n'obtiennent pas la même clé.
- **Nonce** : « number used once », valeur différente à chaque message, pour que deux
  chiffrements du même texte ne se ressemblent pas.
- **AES-256-GCM** : algorithme qui chiffre **et** authentifie. Si un seul caractère du
  message est modifié, le déchiffrement échoue au lieu de rendre n'importe quoi.
- **DPAPI** : coffre intégré à Windows, utilisé ici pour ranger votre phrase secrète.
- **RegisterHotKey** : fonction de Windows qui réserve une combinaison de touches.
  Contrairement à un *hook* clavier, elle ne fait pas examiner toutes les touches
  tapées sur la machine.
- **Compilation croisée** : fabriquer, depuis un système, un programme destiné à un
  autre. C'est ce qui permet de produire le `.exe` depuis un Mac.

## Où sont rangés les réglages

`%APPDATA%\CryptoBulle\config.json`

La phrase secrète y est **chiffrée par Windows (DPAPI)** : un autre compte Windows,
ou le même fichier copié sur une autre machine, ne peut pas la lire. Elle n'apparaît
jamais en clair. Le fichier est celui de la version Python : vos réglages sont
conservés.

## Limites, dites honnêtement

- La sécurité repose sur votre **phrase secrète**, pas sur le secret du format.
  Prenez une phrase longue ; « 1234 » se devine.
- Pour échanger des messages, votre correspondant a besoin de CryptoBulle **et** de
  la même phrase secrète. C'est du chiffrement symétrique : une seule clé, partagée.
- Le texte déchiffré passe par le presse-papiers de Windows ; un autre logiciel qui
  le surveille pourrait le voir.
- Windows uniquement. Raccourcis globaux, DPAPI, icône de notification et démarrage
  automatique passent tous par des fonctions propres à Windows.
- L'interface est dessinée directement avec l'API Windows, sans bibliothèque
  graphique. C'est ce qui garde l'exécutable à 2,86 Mo au lieu des 150 Mo et plus
  d'une application à base de navigateur intégré, mais chaque écran est du code
  écrit à la main.

## Développement

```
cmd/cryptobulle/      point d'entrée, manifeste et icône de l'exécutable
internal/crypto/      les deux formats : scrypt, AES-GCM, chiffrement par flux
internal/hotkey/      lecture des combinaisons (« ctrl+alt+d »)
internal/config/      réglages JSON, phrase secrète protégée par DPAPI
internal/w32/         appels aux API de Windows, dessin lissé
internal/app/         boucle de messages, bulle, réglages, icône, frappe masquée
scripts/              build.sh (compilation croisée), make_icon.py (icône)
```

Tests et analyse statique, exécutables sur n'importe quel système :

```bash
go test ./...
go vet ./...
GOOS=windows go vet ./...
```

Les paquets `crypto`, `hotkey` et `config` sont portables et couverts par des tests.
Le code Windows est vérifié à la compilation croisée et par `go vet`.

L'icône est un cadenas dessiné par `scripts/make_icon.py`, qui n'utilise que la
bibliothèque standard de Python : le fichier `.ico` produit fait 4,6 Ko et contient
des images PNG de 16 à 256 pixels.
