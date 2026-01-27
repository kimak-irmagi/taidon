# Agent instructions on documentation

1. When working on the Mermaid diagrams, always quote the complex state names to avoid parsing issues. I.e. `STATE[Complex state name (with punctuation, " or whitespace)]` -> `STATE["Complex state name (with punctuation, \" or whitespace)"]`
2. When adding, updating, or removing the markdown documents in this folder, make
   sure both English (X.md) and Russian (X.RU.md) are consistently updated.
   - Exceptions are the ./user-guides/, ./api-guides/, and ./adr/ subfolders: those
     are to be maintained in English only.
3. Whenever a document is moved, renamed, or deleted, verify the inbound links to that
   document from all the documents and code comments, and update accordingly.
   If the document referenced from elsewhere is removed, ask for the user confirmaiton first.
