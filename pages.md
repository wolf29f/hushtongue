Menu d'entrée

Page de traduction =>
- défaut con -> original
- possibilité de changer la direction avec shit-tab. Si déjà une saisie, elle est conservé et le mot cible actuellement choisi est utilisé
- durant la saisie du texte source, auto-complet de 5 items (si plus, ajouter un ... en dernier)
  - possibilité de défiler les mots existant avec up/down et revenir à la saisie en cours
  - si a des traduction mot en vert
  - si mot connus, non traduit en orange
  - si mot inconnus, rouge
- un mot est saisie sur espace/entrer
- tab permet de passer du mode saisie au mode choix de traduction
- en mode choix de traduction
  - les mots ayant une traduction ont un scroller vert qui permet de choisir par décallage la cible
  - les mots inconnus sont conservé en rouge
  - les mots enregistré, mais non traduits, sont en orange
  - dans tout les cas, dernier item de la liste = un + pour ajouter une traduction
    - si joueurs, saisie libre, toujours de type "root", avec auto-complet des mots connus
    - si MJ, menu pour choisir entre saisie libre (type `root`), ou génération (prendra en compte les préfix/sufixe et root)
- ctrl-c ou ctrl-v pour copier/coller le texte (soit à traduite, soit traduti, selon ou est l'utilisateur). ctrl-c copier la traduction, ctrl-v colle du texte à traduire

Page dictionnaire =>
- liste les mots, par défautl mode con
- switch de dictionnaire avec tab
- choix d'un mot avec entrer
  - permet de voir les traduction du mot
  - permet d'en ajouter à la main
  - permet d'en supprimer à la main (ne supprime que le lien de traduction, pas la cible)
- premier element de la liste: ajouter un mot
- auto-complet pour rechercher un mot sur un ctrl-f
- edition/création de mot
  - si MJ, possibilité de changer le type prefix/root/sufix
  - possibilité de modifier le mot (ne change pas le lien de traduction) => vérifier que le nouveau mots n'exist pas. S'il existe, proposer de fusionner.
  - possibilité de lister les traduction, ajouter ou supprimer
    - possible d'ajouter un mot, sera de type root

Pour la génération:
- on liste toutes les combinatoire possible de prefix/root/suffix en séparrant les blocs clairement "anti-voiture-ette"
- l'utilisateur peux choisir une base. Code couleur classique
  - connus => vert
  - existe, sans traduction => orange
  - inconnus => rouge (pour le root seulement du coup)
- une fois la base choisie, l'utilisateur peux choisir entre plusieur traduction pour chaque morceau au besoin
- il peux générer un mot pour root si besoin (les prefix/suffix sont généré/définit à part depuis le dictionnaire, pour simpliffier)
- Si pas de décomposition (cas général) => possibilité de générer ou choisir une traduction.