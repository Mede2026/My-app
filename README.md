# CryptoBulle

**Chiffrer et déchiffrer du texte n'importe où dans Windows, avec deux raccourcis clavier.**

Vous sélectionnez du texte, vous faites `Ctrl+Alt+E` : le texte est remplacé par un
message chiffré. Vous sélectionnez ce message, vous faites `Ctrl+Alt+D` : une petite
**bulle** apparaît près de la souris et affiche le texte d'origine.

---

## Ce que ça fait

| Raccourci (modifiable) | Action |
| --- | --- |
| `Ctrl+Alt+E` | Chiffre la sélection et la colle à la place |
| `Ctrl+Alt+D` | Déchiffre la sélection et l'affiche dans une bulle |

- L'application vit dans la **zone de notification** (à côté de l'horloge). Clic droit :
  réglages, chiffrer, déchiffrer, quitter.
- La bulle se ferme toute seule après quelques secondes, sauf si la souris est dessus.
  Boutons **Copier** et **Fermer**, touche `Échap`, déplaçable à la souris.
- L'ancien contenu du presse-papiers est remis en place après l'opération.
- Un onglet **Atelier** permet de chiffrer ou déchiffrer à la main, sans raccourci.

## Installation rapide (Windows)

```bat
git clone https://github.com/mede2026/my-app.git
cd my-app
build_windows.bat
```

Le fichier `dist\CryptoBulle.exe` est autonome : double-cliquez dessus. Au premier
lancement, la fenêtre de réglages demande votre **phrase secrète**.

Sans compilation, pour tester tout de suite :

```bat
python -m venv .venv
.venv\Scripts\pip install -r requirements.txt
.venv\Scripts\pythonw cryptobulle.pyw
```

Le dépôt contient aussi une **action GitHub** qui construit le `.exe` à chaque
push : onglet *Actions* → dernier build → artefact `CryptoBulle-windows`.

## Le format « MC1 », propre à l'application

Un message chiffré ressemble à ceci :

```
MC1~AfhmdLEr3GMxg2S_a91UZnTlHOI5KYzevFCwNQ4tWV7cpXRbDuykjJsoi6q80B...
```

Ce que fait CryptoBulle, dans l'ordre :

1. **scrypt** transforme votre phrase secrète (+ une constante propre à l'application,
   le « poivre ») en une clé de 256 bits.
2. **AES-256-GCM** chiffre le texte avec cette clé, un **sel** et un **nonce** tirés au hasard.
3. Le résultat est encodé avec un **alphabet maison** : une permutation des 64 caractères
   du base64, unique à CryptoBulle.

Conséquence : un décodeur base64 en ligne ne renvoie que du bruit, et même le bon
outil ne sert à rien sans la bonne phrase secrète.

### Petit lexique

- **Chiffrer** : rendre un texte illisible sans la clé. (« Encrypter » est un anglicisme.)
- **Clé** : la suite de bits qui verrouille et déverrouille le message.
- **scrypt** : fonction qui fabrique une clé à partir d'un mot de passe, en étant
  volontairement lente et gourmande en mémoire, pour décourager les essais en masse.
- **Sel** : valeur aléatoire ajoutée avant scrypt, pour que deux personnes avec la même
  phrase secrète n'aient pas la même clé.
- **Nonce** : « number used once », valeur aléatoire différente à chaque message, pour que
  deux chiffrements du même texte ne se ressemblent pas.
- **AES-256-GCM** : algorithme qui chiffre **et** authentifie. Si un seul caractère du
  message est modifié, le déchiffrement échoue au lieu de rendre n'importe quoi.
- **DPAPI** : coffre intégré à Windows, utilisé ici pour ranger votre phrase secrète.

## Où sont rangés les réglages

`%APPDATA%\CryptoBulle\config.json`

La phrase secrète y est **chiffrée par Windows (DPAPI)** : un autre compte Windows, ou
le même fichier copié sur une autre machine, ne peut pas la lire. Elle n'apparaît jamais
en clair dans le fichier.

## Limites, dites honnêtement

- La sécurité repose sur votre **phrase secrète**, pas sur le secret du format. Prenez
  une phrase longue ; « 1234 » se devine.
- Pour échanger des messages avec quelqu'un, cette personne doit avoir CryptoBulle **et**
  exactement la même phrase secrète (c'est du chiffrement symétrique, une seule clé partagée).
- Le texte déchiffré passe par le presse-papiers de Windows ; un autre logiciel qui
  surveille le presse-papiers pourrait le voir.
- Windows uniquement : les raccourcis globaux, DPAPI et le démarrage automatique passent
  par des interfaces propres à Windows.

## Développement

```
cryptobulle/
  crypto.py       chiffrement, déchiffrement, détection des jetons MC1~
  config.py       réglages JSON
  secretstore.py  phrase secrète protégée par DPAPI
  clipboard.py    lecture de la sélection, collage
  hotkeys.py      raccourcis globaux
  bubble.py       la bulle Tkinter
  ui_settings.py  fenêtre de réglages + atelier
  tray.py         icône près de l'horloge
  app.py          assemblage
```

Tests (aucune dépendance Windows nécessaire) :

```bash
pip install cryptography
python -m unittest discover -s tests
```
