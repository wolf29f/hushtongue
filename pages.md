Menu d'entrée

Page de traduction =>
- direction par défaut : con -> original si joueur, original -> con si MJ
- un bouton change la direction. Changer de direction efface la saisie (modale de confirmation si une saisie est en cours)
- durant la saisie du texte source, auto-complet de 5 items (si plus, ajouter un ... en dernier)
  - possibilité de défiler les mots existant avec up/down et revenir à la saisie en cours
  - si a des traduction mot en vert
  - si mot connus, non traduit en orange
  - si mot inconnus, rouge
- un mot est saisie sur espace/entrer
- tab/shift-tab changent le focus entre la saisie, le choix de traduction et le bouton de direction : passer de la saisie au choix de traduction change de mode
- en mode choix de traduction
  - gauche/droite pour passer d'un mot à l'autre, haut/bas pour changer la traduction du mot
  - les mots ayant une traduction ont un scroller vert qui permet de choisir par décallage la cible
  - les mots inconnus sont conservé en rouge
  - les mots enregistré, mais non traduits, sont en orange
  - dans tout les cas, dernier item de la liste = un + pour ajouter une traduction
    - si joueurs, saisie libre, toujours de type "root", avec auto-complet des mots connus
    - si MJ, menu pour choisir entre saisie libre (type `root`), ou génération (prendra en compte les préfix/sufixe et root)
- ctrl-c ou ctrl-v pour copier/coller le texte (soit à traduite, soit traduti, selon ou est l'utilisateur). ctrl-c copier la traduction, ctrl-v colle du texte à traduire

Page dictionnaire =>
- liste les mots, langue par défaut : con si joueur, original si MJ
- un bouton change de langue (tab/shift-tab passent d'un élément à l'autre)
- un bouton "Ajouter" pour ajouter un mot
- recherche avec `/` (filtre de la liste)
- choix d'un mot avec entrer => fiche du mot
  - liste les traductions du mot. Entrer sur une traduction ouvre la fiche de ce mot
  - ajout d'une traduction à la main : choix d'un mot existant de l'autre langue, ou saisie d'un nouveau mot (sera de type `root`)
  - suppression d'une traduction (ne supprime que le lien de traduction, pas la cible)
  - si MJ et mot original : génération d'une traduction (voir plus bas)
  - modification du mot (ne change pas le lien de traduction). Si le nouveau mot existe déjà, proposition de fusionner : les traductions sont regroupées et l'orthographe du mot existant est conservée
  - si MJ, possibilité de changer le type prefix/root/suffix. Supprime les traductions du mot (modale de confirmation s'il en a)
  - suppression du mot (modale de confirmation), supprime aussi ses liens de traduction

Pour la génération (MJ, mot original, depuis la fiche du mot ou le choix de traduction):
- on liste toutes les décompositions du mot en prefix/root/suffix connus, en séparant les blocs clairement "anti-voiture-ette". Le mot entier comme root en fait partie
- chaque décomposition est combinée avec les traductions possibles de chaque morceau : une ligne par combinaison
- tri : traduction complète existante en premier, puis par nombre de morceaux déjà traduits, génération pure en dernier
- code couleur de la partie con
  - traduction existante => vert
  - générée => rouge
- un morceau sans traduction (root inconnu, ou mot connu non traduit) reçoit un mot généré. La génération est déterministe : un même mot donne toujours le même résultat, sauf si ce résultat existe déjà parmi les mots con du même type (on passe alors à une variante)
- valider une ligne crée les mots et liens manquants. Avec plusieurs morceaux, le mot est aussi lié au mot con composé (de type `root`)
