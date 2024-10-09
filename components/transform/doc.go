// Package transform provides data transformation components for the RuleGo rule engine.
//
// These components are designed to modify or convert data within the rule chain:
//
// - ExprTransformNode: Transforms data using expression language
// - JsTransformNode: Transforms data using JavaScript
// - TemplateNode: Transforms data using a text/template
//
// Each component is registered with the Registry, allowing them to be used
// within rule chains. These components enable complex data manipulations and
// format conversions as part of rule processing flows.
//
// To use these components, include them in your rule chain configuration and
// configure them to perform the desired data transformations on messages
// passing through the rule chain.
//
// You can use these components in your rule chain DSL file by referencing
// their Type. For example:
//
//	{
//	  "id": "node1",
//	  "type": "jsTransform",
//	  "name": "js transform",
//	  "configuration": {
//	    "jsScript": "metadata['state']='modify by js';\n msg['addField']='addValueFromJs'; return {'msg':msg,'metadata':metadata,'msgType':msgType};"
//	  }
//	}
package transform
