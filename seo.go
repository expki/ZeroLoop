package main

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"path"
	"strings"
)

// createRobotsTxtHandler returns a handler that generates robots.txt dynamically
func createRobotsTxtHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var builder strings.Builder
		builder.WriteString("# Agent Zero")
		builder.WriteString("\n## A minimal agent loop that traps LLMs in a cycle of thinking, executing, observing, and repeating.\n")

		// User-agent
		builder.WriteString("\nUser-agent: *\n")

		// Static allowed pages
		builder.WriteString("\n# Static pages\n")
		// TODO: add static pages

		// Dynamic service pages
		// TODO: add dynamic pages

		// Disallowed paths
		builder.WriteString("\n# Private/admin areas\n")
		// TODO: add disallowed paths

		// Sitemap reference
		fmt.Fprintf(&builder, "\nSitemap: %s/sitemap.xml\n", r.Host)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write([]byte(builder.String()))
	}
}

// createLlmsTxtHandler returns a handler that generates llms.txt dynamically
func createLlmsTxtHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var builder strings.Builder
		builder.WriteString("# Agent Zero\n")
		builder.WriteString("\n> A minimal agent loop that traps LLMs in a cycle of thinking, executing, observing, and repeating.\n")

		// TODO: add sections

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write([]byte(builder.String()))
	}
}

// URLSet represents the root element of sitemap.xml
type URLSet struct {
	XMLName xml.Name `xml:"urlset"`
	XMLNS   string   `xml:"xmlns,attr"`
	URLs    []URL    `xml:"url"`
}

// URL represents a single URL entry in sitemap.xml
type URL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

// createSitemapXmlHandler returns a handler that generates sitemap.xml dynamically
func createSitemapXmlHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		urlset := URLSet{
			XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		}

		// Pages
		staticPages := []struct {
			path       string
			changeFreq string // "monthly", "weekly", "daily"
			priority   string // "1.0" to "0.1"
		}{
			// TODO: add pages
		}
		for _, page := range staticPages {
			urlset.URLs = append(urlset.URLs, URL{
				Loc:        path.Join(r.Host, page.path),
				ChangeFreq: page.changeFreq,
				Priority:   page.priority,
			})
		}

		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write([]byte(xml.Header))
		encoder := xml.NewEncoder(w)
		encoder.Indent("", "  ")
		encoder.Encode(urlset)
	}
}
