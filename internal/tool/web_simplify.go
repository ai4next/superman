package tool

import (
	"fmt"
	"strings"
)

const genericAgentOptHTMLJS = `
function optHTML(text_only=false) {
  function createEnhancedDOMCopy() {
    const ignoreTags = ['SCRIPT', 'STYLE', 'NOSCRIPT', 'META', 'LINK', 'COLGROUP', 'COL', 'TEMPLATE', 'PARAM', 'SOURCE'];
    const ignoreIds = ['ljq-ind'];
    function cloneNode(sourceNode, keep=false) {
      if (sourceNode.nodeType === 8 ||
          (sourceNode.nodeType === 1 &&
           (ignoreTags.includes(sourceNode.tagName) || (sourceNode.id && ignoreIds.includes(sourceNode.id))))) {
        return null;
      }
      if (sourceNode.nodeType === 3) return sourceNode.cloneNode(false);
      if (sourceNode.nodeType !== 1) return null;

      const clone = sourceNode.cloneNode(false);
      if ((sourceNode.tagName === 'INPUT' || sourceNode.tagName === 'TEXTAREA') && sourceNode.value) clone.setAttribute('value', sourceNode.value);
      if (sourceNode.tagName === 'INPUT' && (sourceNode.type === 'radio' || sourceNode.type === 'checkbox') && sourceNode.checked) clone.setAttribute('checked', '');
      else if (sourceNode.tagName === 'SELECT' && sourceNode.value) clone.setAttribute('data-selected', sourceNode.value);
      try {
        if (sourceNode.matches && sourceNode.matches(':-webkit-autofill')) {
          clone.setAttribute('data-autofilled', 'true');
          if (!sourceNode.value) clone.setAttribute('value', 'protected-autofill-value');
        }
      } catch(e) {}

      const isDropdown = sourceNode.classList?.contains('dropdown-menu') ||
        /dropdown|menu/i.test(sourceNode.className || '') || sourceNode.getAttribute('role') === 'menu';
      const ddItems = isDropdown ? sourceNode.querySelectorAll('a, button, [role="menuitem"], li').length : 0;
      const isSmallDropdown = ddItems > 0 && ddItems <= 7 && sourceNode.textContent.length < 500;

      const childNodes = [];
      for (const child of sourceNode.childNodes) {
        const childClone = cloneNode(child, keep || isSmallDropdown);
        if (childClone) childNodes.push(childClone);
      }
      if (sourceNode.tagName === 'IFRAME') {
        try {
          const iDoc = sourceNode.contentDocument || sourceNode.contentWindow?.document;
          if (iDoc && iDoc.body && iDoc.body.children.length > 0) {
            const wrapper = document.createElement('div');
            wrapper.setAttribute('data-iframe-content', sourceNode.src || '');
            for (const ch of iDoc.body.childNodes) {
              const c = cloneNode(ch, keep);
              if (c) wrapper.appendChild(c);
            }
            if (wrapper.childNodes.length) childNodes.push(wrapper);
          }
        } catch(e) {}
      }
      if (sourceNode.shadowRoot) {
        for (const shadowChild of sourceNode.shadowRoot.childNodes) {
          const shadowClone = cloneNode(shadowChild, keep);
          if (shadowClone) childNodes.push(shadowClone);
        }
      }

      const rect = sourceNode.getBoundingClientRect();
      const style = window.getComputedStyle(sourceNode);
      const isVisible = (rect.width > 1 && rect.height > 1 &&
        style.display !== 'none' && style.visibility !== 'hidden' &&
        parseFloat(style.opacity) > 0 &&
        Math.abs(rect.left) < 5000 && Math.abs(rect.top) < 5000) || isSmallDropdown;
      const hasElementChildren = childNodes.some(child => child.nodeType !== 3);

      if (sourceNode.tagName === 'DIV' && !hasElementChildren && !sourceNode.textContent.trim()) return null;
      if (sourceNode.getAttribute('aria-hidden') === 'true' && !isVisible) return null;
      if (isVisible || hasElementChildren || keep) {
        childNodes.forEach(child => clone.appendChild(child));
        return clone;
      }
      return null;
    }
    return cloneNode(document.body);
  }

  const domCopy = createEnhancedDOMCopy();
  if (!domCopy) return '';
  if (text_only) {
    const blocks = new Set(['DIV','P','H1','H2','H3','H4','H5','H6','LI','TR','SECTION','ARTICLE','HEADER','FOOTER','NAV','BLOCKQUOTE','PRE','HR','BR','DT','DD','FIGCAPTION','DETAILS','SUMMARY']);
    domCopy.querySelectorAll('*').forEach(el => { if (blocks.has(el.tagName)) el.insertAdjacentText('beforebegin', '\n'); });
    domCopy.querySelectorAll('input:not([type=hidden]),textarea,select').forEach(el => {
      const p = [el.tagName, el.id && '#' + el.id, el.getAttribute('name') && 'name=' + el.getAttribute('name'),
        el.tagName === 'INPUT' && 'type=' + (el.getAttribute('type') || 'text'),
        el.getAttribute('placeholder') && '"' + el.getAttribute('placeholder') + '"',
        el.getAttribute('data-autofilled') && 'autofilled', el.disabled && 'disabled',
        el.tagName === 'SELECT' && el.getAttribute('data-selected') && '="' + el.getAttribute('data-selected') + '"'].filter(Boolean).join(' ');
      el.insertAdjacentText('beforebegin', '\n[' + p + ']\n');
    });
    domCopy.querySelectorAll('button[disabled]').forEach(el => el.insertAdjacentText('beforebegin', '[DISABLED] '));
    return domCopy.textContent.replace(/ {2,}/g, ' ').replace(/^ +/gm, '').replace(/(\n\s*){3,}/g, '\n\n').trim();
  }

  let root = domCopy;
  while (root.children && root.children.length === 1) root = root.children[0];
  for (let i = 0; i < 3; i++) root.querySelectorAll('div').forEach(div => (!div.textContent.trim() && div.children.length === 0) && div.remove());
  root.querySelectorAll('svg').forEach(svg => { svg.textContent = ''; [...svg.attributes].forEach(a => svg.removeAttribute(a.name)); });
  root.querySelectorAll('*').forEach(tag => {
    tag.removeAttribute('style');
    for (const attr of [...tag.attributes]) {
      const n = attr.name, v = attr.value || '';
      if (n === 'src') {
        if (v.startsWith('data:')) tag.setAttribute(n, '__img__');
        else if (v.length > 30) tag.setAttribute(n, '__url__');
      } else if ((n === 'href' || n === 'action') && v.length > 30) tag.setAttribute(n, n === 'href' ? '__link__' : '__url__');
      else if ((n === 'value' || n === 'title' || n === 'alt') && v.length > 100) tag.setAttribute(n, v.slice(0, 50) + ' ...');
      else if (!['id','class','name','src','href','alt','value','type','placeholder','disabled','checked','selected','readonly','required','multiple','role','aria-label','aria-expanded','aria-hidden','contenteditable','title','for','action','method','target','colspan','rowspan'].includes(n)) {
        if (n.startsWith('data-v')) tag.removeAttribute(n);
        else if (n.startsWith('data-') && v.length > 20) tag.setAttribute(n, '__data__');
        else if (!n.startsWith('data-')) tag.removeAttribute(n);
      }
    }
  });
  return root.outerHTML;
}
`

const genericAgentTempMonitorJS = `
function startStrMonitor(interval) {
  if (window._tm && window._tm.id) clearInterval(window._tm.id);
  window._tm = {extract: () => {
    const texts = new Set(), walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    let node, t, s; while (node = walker.nextNode())
      ((t = node.textContent.trim()) && t.length > 10 && !(s = t.substring(0, 20)).includes('_')) && texts.add(s);
    return texts;
  }};
  window._tm.init = window._tm.extract();
  window._tm.all = new Set();
  window._tm.id = setInterval(() => window._tm.extract().forEach(t => window._tm.all.add(t)), interval);
}
startStrMonitor(450);
`

const genericAgentGetTempTextsJS = `
function stopStrMonitor() {
  if (!window._tm) return [];
  clearInterval(window._tm.id);
  const final = window._tm.extract();
  const newlySeen = [...window._tm.all].filter(t => !window._tm.init.has(t));
  let result;
  if (newlySeen.length < 8) result = newlySeen;
  else result = newlySeen.filter(t => !final.has(t));
  delete window._tm;
  return result;
}
return stopStrMonitor();
`

func genericAgentScanScript(textOnly bool) string {
	return genericAgentOptHTMLJS + fmt.Sprintf("\nreturn optHTML(%t);", textOnly)
}

func genericAgentMonitorStartScript() string {
	return genericAgentOptHTMLJS + "\n" + genericAgentTempMonitorJS + "\nreturn optHTML(false);"
}

func genericAgentCurrentHTMLScript() string {
	return genericAgentOptHTMLJS + "\nreturn optHTML(false);"
}

func genericAgentSmartFormat(data string, maxStrLen int, omit string) string {
	if maxStrLen <= 0 || len(data) < maxStrLen+len(omit)*2 {
		return data
	}
	half := maxStrLen / 2
	return data[:half] + omit + data[len(data)-half:]
}

func normalizeTextOnlyContent(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(s)
}
