package toolcall

import "strings"

// BuildToolCallInstructions generates the unified tool-calling instruction block
// used by all adapters (OpenAI, Claude, Gemini).
//
// The toolNames slice should contain the actual tool names available in the
// current request; the function picks real names for examples.
func BuildToolCallInstructions(toolNames []string) string {
	return `When you need to call a tool, format it like this:

<|DSML|tool_calls>
<|DSML|invoke name="TOOL_NAME_HERE">
<|DSML|parameter name="PARAMETER_NAME"><![CDATA[PARAMETER_VALUE]]></|DSML|parameter>
</|DSML|invoke>
</|DSML|tool_calls>

Important details to get right:

• Always begin with <|DSML|tool_calls>. Without it, the system won't recognize your call.
• Use <|DSML|invoke name="..."> for each tool you want to run. You can include multiple invocations within a single tool_calls block.
• Every parameter goes inside a <|DSML|parameter name="...">...</|DSML|parameter> element.
• Wrap all text values in CDATA: <![CDATA[your text here]]>. Yes, even short strings, filenames, code, and queries.
• Numbers, true/false, and null don't need CDATA — write them directly.
• For objects, use nested XML elements. For arrays, repeat <item> for each entry.

What to avoid:

• Don't wrap the tool block in markdown code fences. Just output the raw XML.
• Don't start with <|DSML|invoke> without the outer <|DSML|tool_calls> wrapper.
• Don't include any text after the closing </|DSML|tool_calls>.
• Don't create empty parameters or fill in placeholders. If you don't know a value, ask the user instead.
• For command-line tools (Bash, execute_command), never leave the command parameter empty.
• Only use parameter names that appear in the tool's schema. Don't make up new ones.
• If a required parameter is missing information, respond normally and ask for it rather than guessing.

Parameter type reference:

• text values: <|DSML|parameter name="x"><![CDATA[value]]></|DSML|parameter>
• nested objects: <|DSML|parameter name="x"><field>...</field></|DSML|parameter>
• arrays / lists: <|DSML|parameter name="x"><item>...</item><item>...</item></|DSML|parameter>
• numbers, booleans, null: <|DSML|parameter name="x">plain_text</|DSML|parameter>

The older XML format <tool_calls>/<invoke>/<parameter> also works, but the DSML version above is preferred.

` + buildCorrectToolExamples(toolNames) + `
Place your tool call block at the end of your response. That is the correct position for it.`
}

type promptToolExample struct {
	name   string
	params string
}

func buildCorrectToolExamples(toolNames []string) string {
	names := uniqueToolNames(toolNames)
	examples := make([]string, 0, 4)

	if single, ok := firstBasicExample(names); ok {
		examples = append(examples, "Single tool call:\n"+renderToolExampleBlock([]promptToolExample{single}))
	}
	if parallel := firstNBasicExamples(names, 2); len(parallel) >= 2 {
		examples = append(examples, "Multiple tools at once:\n"+renderToolExampleBlock(parallel))
	}
	if nested, ok := firstNestedExample(names); ok {
		examples = append(examples, "Tool with nested parameters:\n"+renderToolExampleBlock([]promptToolExample{nested}))
	}
	if script, ok := firstScriptExample(names); ok {
		examples = append(examples, "Tool with a script using CDATA:\n"+renderToolExampleBlock([]promptToolExample{script}))
	}

	if len(examples) == 0 {
		return ""
	}
	return "Correct usage patterns:\n\n" + strings.Join(examples, "\n\n") + "\n\n"
}

func uniqueToolNames(toolNames []string) []string {
	names := make([]string, 0, len(toolNames))
	seen := map[string]bool{}
	for _, name := range toolNames {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

func firstBasicExample(names []string) (promptToolExample, bool) {
	for _, name := range names {
		if params, ok := exampleBasicParams(name); ok {
			return promptToolExample{name: name, params: params}, true
		}
	}
	return promptToolExample{}, false
}

func firstNBasicExamples(names []string, count int) []promptToolExample {
	out := make([]promptToolExample, 0, count)
	for _, name := range names {
		if params, ok := exampleBasicParams(name); ok {
			out = append(out, promptToolExample{name: name, params: params})
			if len(out) == count {
				return out
			}
		}
	}
	return out
}

func firstNestedExample(names []string) (promptToolExample, bool) {
	for _, name := range names {
		if params, ok := exampleNestedParams(name); ok {
			return promptToolExample{name: name, params: params}, true
		}
	}
	return promptToolExample{}, false
}

func firstScriptExample(names []string) (promptToolExample, bool) {
	for _, name := range names {
		if params, ok := exampleScriptParams(name); ok {
			return promptToolExample{name: name, params: params}, true
		}
	}
	return promptToolExample{}, false
}

func renderToolExampleBlock(calls []promptToolExample) string {
	var b strings.Builder
	b.WriteString("<|DSML|tool_calls>\n")
	for _, call := range calls {
		b.WriteString(`  <|DSML|invoke name="`)
		b.WriteString(call.name)
		b.WriteString(`">` + "\n")
		b.WriteString(indentPromptParameters(call.params, "    "))
		b.WriteString("\n  </|DSML|invoke>\n")
	}
	b.WriteString("</|DSML|tool_calls>")
	return b.String()
}

func indentPromptParameters(body, indent string) string {
	if strings.TrimSpace(body) == "" {
		return indent + `<|DSML|parameter name="content"></|DSML|parameter>`
	}
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = line
			continue
		}
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

func wrapParameter(name, inner string) string {
	return `<|DSML|parameter name="` + name + `">` + inner + `</|DSML|parameter>`
}

func exampleBasicParams(name string) (string, bool) {
	switch strings.TrimSpace(name) {
	case "Read":
		return wrapParameter("file_path", promptCDATA("README.md")), true
	case "Glob":
		return wrapParameter("pattern", promptCDATA("**/*.go")) + "\n" + wrapParameter("path", promptCDATA(".")), true
	case "read_file":
		return wrapParameter("path", promptCDATA("src/main.go")), true
	case "list_files":
		return wrapParameter("path", promptCDATA(".")), true
	case "search_files":
		return wrapParameter("query", promptCDATA("tool call parser")), true
	case "Bash", "execute_command":
		return wrapParameter("command", promptCDATA("pwd")), true
	case "exec_command":
		return wrapParameter("cmd", promptCDATA("pwd")), true
	case "Write":
		return wrapParameter("file_path", promptCDATA("notes.txt")) + "\n" + wrapParameter("content", promptCDATA("Hello world")), true
	case "write_to_file":
		return wrapParameter("path", promptCDATA("notes.txt")) + "\n" + wrapParameter("content", promptCDATA("Hello world")), true
	case "Edit":
		return wrapParameter("file_path", promptCDATA("README.md")) + "\n" + wrapParameter("old_string", promptCDATA("foo")) + "\n" + wrapParameter("new_string", promptCDATA("bar")), true
	case "MultiEdit":
		return wrapParameter("file_path", promptCDATA("README.md")) + "\n" + `<|DSML|parameter name="edits"><item><old_string>` + promptCDATA("foo") + `</old_string><new_string>` + promptCDATA("bar") + `</new_string></item></|DSML|parameter>`, true
	}
	return "", false
}

func exampleNestedParams(name string) (string, bool) {
	switch strings.TrimSpace(name) {
	case "MultiEdit":
		return wrapParameter("file_path", promptCDATA("README.md")) + "\n" + `<|DSML|parameter name="edits"><item><old_string>` + promptCDATA("foo") + `</old_string><new_string>` + promptCDATA("bar") + `</new_string></item></|DSML|parameter>`, true
	case "Task":
		return wrapParameter("description", promptCDATA("Investigate flaky tests")) + "\n" + wrapParameter("prompt", promptCDATA("Run targeted tests and summarize failures")), true
	case "ask_followup_question":
		return wrapParameter("question", promptCDATA("Which approach do you prefer?")) + "\n" + `<|DSML|parameter name="follow_up"><item><text>` + promptCDATA("Option A") + `</text></item><item><text>` + promptCDATA("Option B") + `</text></item></|DSML|parameter>`, true
	}
	return "", false
}

func exampleScriptParams(name string) (string, bool) {
	scriptCommand := `cat > /tmp/test_escape.sh <<'EOF'
#!/bin/bash
echo 'single "double"'
echo "literal dollar: \$HOME"
EOF
bash /tmp/test_escape.sh`
	scriptContent := `#!/bin/bash
echo 'single "double"'
echo "literal dollar: $HOME"`

	switch strings.TrimSpace(name) {
	case "Bash":
		return wrapParameter("command", promptCDATA(scriptCommand)) + "\n" + wrapParameter("description", promptCDATA("Test shell escaping")), true
	case "execute_command":
		return wrapParameter("command", promptCDATA(scriptCommand)), true
	case "exec_command":
		return wrapParameter("cmd", promptCDATA(scriptCommand)), true
	case "Write":
		return wrapParameter("file_path", promptCDATA("test_escape.sh")) + "\n" + wrapParameter("content", promptCDATA(scriptContent)), true
	case "write_to_file":
		return wrapParameter("path", promptCDATA("test_escape.sh")) + "\n" + wrapParameter("content", promptCDATA(scriptContent)), true
	}
	return "", false
}

func promptCDATA(text string) string {
	if text == "" {
		return ""
	}
	if strings.Contains(text, "]]>") {
		return "<![CDATA[" + strings.ReplaceAll(text, "]]>", "]]]]><![CDATA[>") + "]]>"
	}
	return "<![CDATA[" + text + "]]>"
}
