# CryptoBulle

**Chiffrer et déchiffrer du texte n'importe où dans Windows, avec deux raccourcis clavier.**

Vous sélectionnez du texte, vous faites `Ctrl+Alt+E` : le texte est remplacé par un
message chiffré. Vous sélectionnez ce message, vous faites `Ctrl+Alt+D` : une petite
**bulle** apparaît près de la souris et affiche le texte d'origine.

Un seul fichier `CryptoBulle.exe` de **2,86 Mo**. Rien à installer.

---

## Ce que ça fait

| Raccourci (modifiable) | Action |
| --- | --- |
| `Ctrl+Alt+E` | Chiffre la sélection et la colle à la place |
| `Ctrl+Alt+D` | Déchiffre la sélection et l'affiche dans une bulle |

- L'application vit dans la **zone de notification**, à côté de l'horloge. Clic droit :
  réglages, chiffrer, déchiffrer, quitter.
- La bulle se ferme seule après quelques secondes, sauf si la souris est dessus.
  Boutons **Copier** et **Fermer**, touche `Échap`, déplaçable à la souris.
- L'ancien contenu du presse-papiers est remis en place après l'opération.
- La fenêtre de réglages contient un **atelier** pour chiffrer ou déchiffrer à la main.

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

## Le format « MC1 », propre à l'application

Un message chiffré ressemble à ceci :

```
MC1~UZgbAzkdfBpOmBiKoUQ5b0pfhpzwi5CZFsCwIO85HMFHwZpipPP6tp06OhG18Wmn...
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
internal/crypto/      format MC1 : scrypt, AES-GCM, jetons, détection
internal/hotkey/      lecture des combinaisons (« ctrl+alt+d »)
internal/config/      réglages JSON, phrase secrète protégée par DPAPI
internal/w32/         appels aux API de Windows, dessin lissé
internal/app/         boucle de messages, bulle, réglages, icône, actions
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
