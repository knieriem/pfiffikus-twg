Twg is the tiny website generator once built for the simple static website of a non-profit association.

## Structure of a website project

-	`public/`	- will contain the result of a build, i.e. the generated static website files

-	`static/`	- may contain CSS files, images, JavaScript files;
	during build they will just be copied into the public folder

-	`content/`	- source files of the individual pages that may either be formatted
	as the <body>-part of a html document,
	or Markdown, depending on the file extension.
	Each file is also processed as a Go template, which allows inserting
	data that should not be stored in a repository.

-	`navigation.txt`	- defines the navigation menu;
	each line contains a page name that will be part of the page's URL,
	and, separated by whitespace,
	the text that shall be displayed in the corresponding navigation menu item.

-	`template.html`	- defines the base template of a complete HTML page
	that is used for each individual page of the website


## View the Website locally

To view the website without explicitely generating it, run

	./twg serve

or just

	./twg

The site, as it would look like when published,
will be served locally at `http://localhost:9999/`.


## Generate website files

	./twg build

This will create the folder `./public`; if this directory already exists, `twg` will fail.
Option `-f` may be used to force `twg` to overwrite the folder `public`, though:

	./twg build -f

## Publish website files

To publish the website,
just copy the contents of the `public/` folder to the remote site.
