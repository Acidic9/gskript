var rawBio = $("#profile-bio").html();
$("#profile-bio").html(markdownToHTML($("#profile-bio").text()));
if ($("#profile-bio").text() === "") {
	$("#profile-bio").html("This user has no biography.");
}

var simplemde = new SimpleMDE();

function editBio() {
	swal({
		title: null,
		text: '<textarea name="bio-editor" id="bio-editor"></textarea>',
		html: true,
		showCancelButton: true,
		closeOnConfirm: false,
  		showLoaderOnConfirm: true,
		confirmButtonText: "Save",
		confirmButtonColor: "#A6E22D",
		customClass: "bio-editor"
	}, function() {
		var bioText = simplemde.value();
		$.ajax({
			url: '/do/update-profile/bio',
			type: 'POST',
			data: {bio: bioText},

			success: function(resp) {
				if (!resp.success) {
					swal({
						title: "Failed to Update Bio",
						text: resp.err,
						type: "error",
						html: true,
					});
					return;
				}
				
				swal.close();
				rawBio = bioText.trim();
				if (rawBio === "") {
					$("#profile-bio").html("This user has no biography.");
				} else {
					$("#profile-bio").html(markdownToHTML(rawBio));
				}
			},

			error: function() {
				swal({
					title: "Failed to Update Bio",
					text: "Something went wrong when updating your bio.",
					type: "error",
				});
			}
		});
	});
	
	simplemde = new SimpleMDE({
		element: document.getElementById("bio-editor"),
		placeholder: "Something about myself...",
		showIcons: ["strikethrough", "heading-1", "heading-2", "heading-3", "code", "unordered-list", "ordered-list", "horizontal-rule", "table"],
		hideIcons: ["side-by-side", "fullscreen", "guide"],
		lineWrapping: true
	});

	simplemde.value(htmlDecode(rawBio));
}

function htmlDecode(input){
	var e = document.createElement('div');
	e.innerHTML = input;
	return e.childNodes.length === 0 ? "" : e.childNodes[0].nodeValue;
}

if(!String.prototype.trim) {  
	String.prototype.trim = function () {  
		return this.replace(/^\s+|\s+$/g,'');  
	};  
}