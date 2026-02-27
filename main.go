package main

import (
	"embed"

	"github.com/jrswab/axe/cmd"
)

//go:embed skills/sample/SKILL.md
var skillsFS embed.FS

func main() {
	cmd.SetSkillsFS(skillsFS)
	cmd.Execute()
}
