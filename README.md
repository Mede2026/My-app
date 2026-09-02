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
- **Aucune dépendance** : ni `keyboard`, ni `pystray`, ni `Pillow`, ni `cryptography`.
  Tout passe par la bibliothèque standard de Python et les API de Windows.
- La bulle se ferme toute seule après quelques secondes, sauf si la souris est dessus.
  Boutons **Copier** et **Fermer**, touche `Échap`, déplaçable à la souris.
- L'ancien contenu du presse-papiers est remis en place après l'opération.
- Un onglet **Atelier** permet de chiffrer ou déchiffrer à la main, sans raccourci.

## Légèreté et vitesse

Tout ce qui pouvait être délégué à Windows l'a été. Résultat : rien à installer,
un exécutable plus petit et un démarrage plus rapide.

| Besoin | Avant (bibliothèques) | Maintenant (Windows) |
| --- | --- | --- |
| AES-256-GCM | `cryptography` | **CNG**, le moteur intégré à Windows |
| Raccourcis globaux | `keyboard` | **RegisterHotKey** |
| Presse-papiers | `pyperclip` | **API presse-papiers** de Windows |
| Icône près de l'horloge | `pystray` + `Pillow` | **Shell_NotifyIcon** |

Vitesses mesurées (les deux premières lignes sur cette machine de test, moteur de secours) :

| Opération | Durée |
| --- | --- |
| Dérivation de la clé, une seule fois au démarrage | 58 ms |
| Chiffrement d'une phrase | 0,23 ms |
| Déchiffrement d'une phrase | 0,01 ms |

Trois choix expliquent ces chiffres :

1. La clé est **calculée à l'avance**, pendant le démarrage, et gardée en mémoire.
   Le raccourci n'attend donc jamais scrypt.
2. La fenêtre-bulle est **construite une fois puis réutilisée**. Le deuxième
   affichage ne coûte plus rien.
3. La copie de la sélection surveille le **numéro de séquence** du presse-papiers,
   un compteur que Windows incrémente à chaque changement. On réagit dès que la
   copie est faite, sans attendre un délai fixe.

Au repos, l'application dort : Windows ne réveille son fil de messages que pour
les deux raccourcis déclarés et pour les clics sur l'icône. Le fil de l'interface
vérifie sa file d'attente toutes les 50 millisecondes, ce qui est négligeable.

**Différence importante avec la version précédente** : `RegisterHotKey` demande à
Windows de surveiller deux combinaisons. La bibliothèque `keyboard`, elle,
installait un *hook* clavier, c'est-à-dire un mouchard qui examinait **chaque
touche** tapée sur la machine, dans tous les logiciels. C'était plus lourd pour le
système et plus intrusif.

## Installation rapide (Windows)

```bat
git clone https://github.com/mede2026/my-app.git
cd my-app
build_windows.bat
```

Vous obtenez `dist\CryptoBulle\CryptoBulle.exe`, la version qui démarre le plus vite.
Pour un fichier unique à transporter sur une clé USB :

```bat
build_windows.bat onefile
```

Un fichier unique est plus pratique, mais Windows doit le décompresser à chaque
lancement : le démarrage est plus lent. À vous de choisir.

Sans rien compiler, avec Python déjà installé :

```bat
pythonw cryptobulle.pyw
```

Aucune installation de paquet n'est nécessaire. Au premier lancement, la fenêtre
de réglages demande votre **phrase secrète**.

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
- **CNG** : « Cryptography API: Next Generation », le moteur cryptographique de
  Windows. C'est lui qui fait le vrai calcul AES, sans bibliothèque à embarquer.
- **ctypes** : module standard de Python qui sait appeler les fonctions des DLL
  de Windows. C'est le pont entre CryptoBulle et le système.
- **Hook clavier** : mouchard qui voit passer toutes les touches tapées sur la
  machine. CryptoBulle n'en utilise pas.

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
- Windows uniquement : raccourcis globaux, CNG, DPAPI et démarrage automatique passent
  tous par des interfaces propres à Windows.
- Tkinter, l'interface graphique standard de Python, reste embarquée dans
  l'exécutable. C'est la dernière grosse pièce ; la retirer voudrait dire dessiner
  la bulle et les réglages en Win32 pur, pour un gain qui ne vaut pas le risque.

## Développement

```
cryptobulle/
  crypto.py       format MC1 : scrypt, jetons, détection dans un texte
  aesgcm.py       AES-256-GCM par Windows CNG, avec test à résultat connu
  config.py       réglages JSON
  secretstore.py  phrase secrète protégée par DPAPI
  winapi.py       déclarations ctypes des API de Windows
  clipboard.py    sélection, presse-papiers, frappes simulées
  hotkeys.py      lecture des combinaisons ("ctrl+alt+d")
  winui.py        boucle de messages : raccourcis + icône près de l'horloge
  bubble.py       la bulle Tkinter, réutilisée d'un affichage à l'autre
  ui_settings.py  fenêtre de réglages + atelier
  app.py          assemblage et échanges entre fils d'exécution
```

Le moteur AES est vérifié au démarrage par un **test à résultat connu** : on
chiffre un message dont le résultat exact est écrit dans le code. Si Windows ne
redonne pas ce résultat, CryptoBulle bascule sur `cryptography` si elle est
présente, sinon il refuse de chiffrer plutôt que de produire n'importe quoi.

Tests (34 tests, exécutables aussi sous Linux ou macOS) :

```bash
pip install cryptography    # moteur de secours, hors Windows uniquement
python -m unittest discover -s tests
```

Sous Windows, la même commande teste le moteur CNG natif, sans rien installer.
