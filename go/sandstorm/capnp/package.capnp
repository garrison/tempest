# Sandstorm - Personal Cloud Sandbox
# Copyright (c) 2014 Sandstorm Development Group, Inc. and contributors
# All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

@0xdf9bc20172856a3a;
# This file contains schemas relevant to the Sandstorm package format.  See also the `spk` tool.

$import "/capnp/c++.capnp".namespace("sandstorm::spk");

using Util = import "util.capnp";
using Powerbox = import "powerbox.capnp";
using Grain = import "grain.capnp";
using ApiSession = import "api-session.capnp".ApiSession;
using Identity = import "identity.capnp";
using WebSession = import "web-session.capnp".WebSession;

struct PackageDefinition {
  id @0 :Text;
  # The app's ID string. This is actually an encoding of the app's public key generated by the spk
  # tool, and looks something like "h37dm17aa89yrd8zuqpdn36p6zntumtv08fjpu8a8zrte7q1cn60".
  #
  # Normally, `spk init` will fill this in for you. You can use `spk keygen` to generate a new ID
  # if needed. The private key corresponding to each ID is stored in a keyring outside your project
  # directory; see `spk help` for more on this.
  #
  # Note that you can specify an alternative ID to `spk pack` with the `-i` flag. This makes sense
  # when you are doing an unofficial build of an app and don't want to use (or don't have access
  # to) the app's real private key.

  manifest @1 :Manifest;
  # Manifest to write as the package's `sandstorm_manifest`.  If null, then `sandstorm-manifest`
  # should appear in the file list.

  sourceMap @2 :SourceMap;
  # Indicates where to search for file to include in the package.

  fileList @3 :Text;
  # Name of a file which itself contains a list of files, one per line, that should be included
  # in the package. Each file should be specified according to its location in the package; the
  # source file will be found by mapping this through `sourceMap`. Each name should be canonical
  # (no ".", "..", or consecutive slashes) and should NOT start with '/'.
  #
  # The file list is automatically generated by `spk dev` based on watching what files are opened
  # by the actual running server. On subsequent runs, new files will be added, but files will never
  # be removed from the list. To reset the list, simply delete it and run `spk dev` again.

  alwaysInclude @4 :List(Text);
  # Files and directories that should always be included in the package whether or not they are
  # in the file named by `fileList`. If you name a directory here, its entire contents will be
  # included recursively (this is not the case in `fileList`). Use this list to name files that
  # wouldn't automatically be included, because for whatever reason the server does not actually
  # open them when running in dev mode. This could include runtime dependencies that are too
  # difficult to test fully, or perhaps a readme file or copyright notice that you want people to
  # see if they unpack your package manually.

  bridgeConfig @5 :BridgeConfig;
  # Configuration variables for apps that use sandstorm-http-bridge.
}

struct Manifest {
  # This manifest file defines an application.  The file `sandstorm-manifest` at the root of the
  # application's `.spk` package contains a serialized (binary) instance of `Manifest`.
  #
  # TODO(soon):  Maybe this should be renamed.  A "manifest" is a list of contents, but this
  #   structure doesn't contain a list at all; it contains information on how to use the contents.

  const sizeLimitInWords :UInt64 = 1048576;
  # The maximum size of the Manifest is 8MB (1M words). This limit is enforced in many places.

  appTitle @7 :Util.LocalizedText;
  # The name of this app as it should be displayed to the user.

  appVersion @4 :UInt32;
  # Among app packages with the same app ID (i.e. the same `publicKey`), `version` is used to
  # decide which packages represent newer vs. older versions of the app.  The sole purpose of this
  # number is to decide whether one package is newer than another; it is not normally displayed to
  # the user.  This number need not have anything to do with the "marketing version" of your app.

  minUpgradableAppVersion @5 :UInt32;
  # The minimum version of the app which can be safely replaced by this app package without data
  # loss.  This might be non-zero if the app's data store format changed drastically in the past
  # and the app is no longer able to read the old format.

  appMarketingVersion @6 :Util.LocalizedText;
  # Human-readable presentation of the app version, e.g. "2.9.17".  This will be displayed to the
  # user to distinguish versions.  It _should_ match the way you identify versions of your app to
  # users in documentation and marketing.

  minApiVersion @0 :UInt32;
  maxApiVersion @1 :UInt32;
  # Min and max API versions against which this app is known to work.  `minApiVersion` primarily
  # exists to warn the user if their instance is too old.  If the sandstorm instance is newer than
  # `maxApiVersion`, it may engage backwards-compatibility hacks and hide features introduced in
  # newer versions.

  metadata @8 :Metadata;
  # Stuff that's not important to actually executing the app, but important to how the app is
  # presented to the user in the Sandstorm UI and app marketplace.

  struct Command {
    # Description of a command to execute.
    #
    # Note that commands specified this way are NOT interpreted by a shell.  If you want shell
    # expansion, you must include a shell binary in your app and invoke it to interpret the
    # command.

    argv @1 :List(Text);
    # Argument list, with the program name as argv[0].

    environ @2 :List(Util.KeyValue);
    # Environment variables to set.  The environment will be completely empty other than what you
    # define here.

    deprecatedExecutablePath @0 :Text;
    # (Obsolete) If specified, will be inserted at the beginning of argv. This is now redundant
    # because you should just specify the program as argv[0]. To be clear, this does not and did
    # never provide a way to make argv[0] contain something other than the executable name, as
    # you can technically do with the `exec` system call.
  }

  struct Action {
    input :union {
      none @0 :Void;
      # This action creates a new grain with no input.

      capability @1 :List(Powerbox.PowerboxDescriptor);
      # This action creates a new grain from a powerbox offer. When a capability matching the query
      # is offered to the user (e.g. by another application calling SessionContext.offer()), this
      # action will be listed as one of the things the user can do with it.
      #
      # On startup, the platform will call create the first session with
      # `UiView.newOfferSession()`.
    }

    command @2 :Command;
    # Command to execute (in a newly-allocated grain) to run this action.

    title @3 :Util.LocalizedText;
    # (Obsolete) Title of this action, to display in the action selector.  This should no longer
    # be used for new apps.

    nounPhrase @5 : Util.LocalizedText;
    # When this action is run, what kind of thing is created? E.g. Etherpad creates a "document".
    # Displayed as "New <nounPhrase>" in the "create new grain" UI.

    description @4 :Util.LocalizedText;
    # Description of this action, suitable for help text.
  }

  actions @2 :List(Action);
  # Actions which this grain offers.

  continueCommand @3 :Command;
  # Command to run to restart an already-created grain.
}

struct SourceMap {
  # Defines where to find files that need to be included in a package.  This is usually combined
  # with a list of files that the package is expected to contain in order to compile a package.
  # The list of files may come from using "spk dev" to

  searchPath @0 :List(Mapping);
  # List of directories to map into the package.

  struct Mapping {
    # Describes a directory to be mapped into the package.

    packagePath @0 :Text;
    # Path where this directory should be mapped into the package.  Must be a canonical file name
    # (no "." nor "..") and must not start with '/'. Omit to map to the package root directory.

    sourcePath @1 :Text;
    # Path on the local system where this directory may be found.  Relative paths are interpreted
    # relative to the location of the package definition file.

    hidePaths @2 :List(Text);
    # Names of files or subdirectories within the directory which should be hidden when mapping
    # this path into the spk.  Use only canonical paths here -- i.e. do not use ".", "..", or
    # multiple consecutive slashes.  Do not use a leading slash.
  }
}

struct BridgeConfig {
  # Configuration variables specific to apps that are using sandstorm-http-bridge. This includes
  # things that need to be communicated to the bridge process before the app starts up, such as
  # permissions.

  viewInfo @0 :Grain.UiView.ViewInfo;
  # What to return from the UiView's getViewInfo(). This structure defines, among other things, the
  # list of shareable permissions and roles that apply to this app. See grain.capnp for more details.
  #
  # When a request comes in from the user, sandstorm-http-bridge will set the
  # X-Sandstorm-Permissions header to a comma-delimited list of permission names corresponding to
  # the user's permissions.

  apiPath @1 :Text;
  # Specifies a path which will be prefixed to all API requests -- that is, requests coming in
  # through the API endpoint as described in:
  #     https://docs.sandstorm.io/en/latest/developing/http-apis/
  #
  # Note that this form of HTTP APIs is old and will eventually be deprecated. In the fantastic
  # future, available APIs should be defined by `powerboxApis`, below. However, as of this writing,
  # `powerboxApis` cannot do everything that old-style HTTP APIs do. Why yes, I did once work at
  # Google, how could you tell?
  #
  # apiPath must end with "/".
  #
  # WARNING: Specifying this does NOT prevent access to other paths. Anyone holding an API token
  # can convert it into a sharing token, which provides full access to the grain's web interface.
  # The purpose of apiPath is only to allow you to design your API URL schema independently of
  # your UI's URL schema, which is often convenient. To actually restrict what an API token
  # holder is allowed to do, you MUST use permissions and enforce them for both API and UI
  # requests. See:
  #     https://docs.sandstorm.io/en/latest/developing/auth/

  saveIdentityCaps @2 :Bool;
  # If true, the first time a new user accesses the grain, the bridge will save the user's Identity
  # capability so that it can be fetched later using `SandstormHttpBridge.getSavedIdentity`. You
  # will probably want to enable this if your app supports notifications.

  expectAppHooks @4 :Bool;
  # Set this to true if you are using sandstorm-http-bridge and want to do any of
  # the following:
  #
  # - Provide an implementation of getViewInfo() that generates its result dynamically,
  #   rather than statically defining the result via `viewInfo` above.
  # - Support exporting persistent capabilities other than the HTTP APIs specified by
  #   PowerboxApi below.
  #
  # If this is true, the bridge will expect the application to establish a capnproto
  # connection to the bridge via `/tmp/sandstorm-api`, and it will expect the app's
  # bootstrap interface on this connection to implement `AppHooks`, defined in
  # sandstorm-http-bridge.capnp. The methods described there can be used to
  # implement the above functionality.

  powerboxApis @3 :List(PowerboxApi);
  struct PowerboxApi {
    # Defines an HTTP API which this application exports, to which other apps can request access
    # via the powerbox.
    #
    # Use this to define APIs to which other applications can request access.
    #
    # Note that this metadata is consulted at the time of a powerbox request. Once the request is
    # complete and a connection has been formed, future changes to the app's manifest will not
    # affect the already-made connection.

    name @0 :Text;
    # Symbolic name for this API. This is passed back to the app as the value of the
    # `X-Sandstorm-Api` header, in all requests made to this API. Note that removing an API
    # definition will NOT revoke existing connections formed through the powerbox. So, the app
    # should be prepared to handle all API names that it has ever defined (or, perhaps, return
    # an appropriate error when it receives a request for an API that no longer exists).

    displayInfo @1 :Powerbox.PowerboxDisplayInfo;
    # Information for display to the user when representing this API.
    #
    # displayInfo.title in particular will be used when displaying a chooser to the user to choose
    # among the app's available APIs.

    path @2 :Text;
    # Specifies the path which will be prefixed to all requests to this API.
    #
    # Like `apiPath` (above), this must end with "/". It is perfectly reasonable for the path to be
    # just "/", if you consider your app's entire HTTP surface to be an API. However, note that
    # unlike with `apiPath`, restricting a powerbox API to a sub-path actually does prevent
    # consumers of the API from accessing other paths. You may wish to use this to your advantage.

    tag @3 :ApiSession.PowerboxTag;
    # Tag defining this API for powerbox matching purposes.

    permissions @4 :Identity.PermissionSet;
    # The permissions represented by this API. A user interacting with the powerbox will not have
    # the option of choosing this API unless they possess at least these permissions. Meanwhile,
    # when a request is made to this API, the `Sandstorm-Permissions` header will always contain
    # exactly these permissions, even if the user who made the powerbox connection has greater
    # permissions.
    #
    # If not specified, the `X-Sandstorm-Permissions` header will contain the exact list of
    # permissions that the user had at the time that they formed the connection. Note that if any
    # of these permissions are later revoked from the user, then the API connection will be revoked
    # in whole.

    # TODO(someday): Allow non-singleton APIs, for grains that contain many objects. Requires app
    #   to implement an embeddable picker UI.
    # TODO(someday): Allow implementing Cap'n Proto APIs via JSON conversion.
  }
}

struct Metadata {
  # Data which is not needed specifically to execute the app, but is useful for purposes like
  # marketing and display.
  #
  # Technically, appMarketingVersion and appTitle belong in this category, but they were defined
  # before Metadata became a thing.
  #
  # NOTE: Any changes here which add new blobs may require updating the front-end so that it
  #   correctly extracts those blobs into separate assets on install.

  icons :group {
    # Various icons to represent the app in various contexts.
    #
    # Each context is associated with a List(Icon). This list contains versions of the same image
    # in varying sizes and formats. When the icon is used, the optimal icon image will be chosen
    # from the list based on the context where it is to be displayed, possibly taking into account
    # parameters like size, display pixel density, and browser image format support.

    appGrid @0 :Icon;
    # The icon shown on the app grid, in the "new grain" flow.
    #
    # Size: 128x128
    # Data size limit: 64kB

    grain @1 :Icon;
    # The icon shown to represent an individual grain, both in the grain table and on the navbar.
    # If omitted, the appGrid icon will be used.
    #
    # Size: 24x24
    # Data size limit: 4kB

    market @2 :Icon;
    # The icon shown in the app market grid. If omitted, the appGrid icon will be used.
    #
    # Size: 150x150
    # Data size limit: 64kB

    marketBig @18 :Icon;
    # The image shown in the app market when visiting the app's page directly, or when featuring
    # a particular app with a bigger display. If omitted, the regular market icon will be used
    # (raster images may look bad).
    #
    # Size: 300x300
    # Data size limit: 256kB
  }

  struct Icon {
    # Represents one icon image.

    union {
      unknown @0 :Void;
      # Unknown file format.

      svg @1 :Text;
      # Scalable Vector Graphics image. This format is *highly* preferred whenever possible.
      #
      # The uncompressed SVG can be up to 4x the documented size limit, to account for the fact
      # that it will be compressed when served.

      png :group {
        # PNG image. You may specify one or both DPI levels.

        dpi1x @2 :Data;
        # Normal-resolution PNG. The image's resolution should exactly match the documented
        # viewport size.

        dpi2x @3 :Data;
        # Double-resolution PNG for high-dpi displays. Any documented data size limit is also
        # doubled for this PNG. (The size limit is only doubled, not quadrupled, because the size
        # of a compressed image should be a function of total interior edge length, not area.)
      }
    }
  }

  website @3 :Text;
  # URL of the app's main web site.

  codeUrl @4 :Text;
  # URL of the app's source code repository, e.g. a Github URL.
  #
  # This field is required if the app's license requires redistributing code (such as the GPL),
  # but is optional otherwise.

  license :group {
    # How is this app licensed?
    #
    # Example usage for open source licenses:
    #
    #     license = (openSource = apache2, notices = (defaultText = embed "notices.txt")),
    #
    # Example usage for proprietary licenses:
    #
    #     license = (proprietary = (defaultText = embed "license.txt"),
    #                notices = (defaultText = embed "notices.txt")),

    union {
      none @5 :Void;
      # No license. Default copyright rules apply; e.g. redistribution is prohibited. See:
      #     http://choosealicense.com/no-license/
      #
      # "None" does NOT mean "public domain". See `publicDomain` below.

      openSource @6 :OpenSourceLicense;
      # Indicates an OSI-approved open source license.
      #
      # If you choose such a license, the license title will be displayed with your app on the app
      # market, and users who specify they want to see only open source apps will see your app.

      proprietary @7 :Util.LocalizedText;
      # Full text of a non-OSI-approved license.
      #
      # Proprietary licenses usually not only restrict copying but also place limitations on *use*
      # of the app. In other words, while open source licenses grant users additional freedoms
      # compared to default copyright rules, proprietary licenses impose additional restrictions.
      #
      # Because of this, the user must explicitly agree to the license. Sandstorm will display the
      # license to the user and ask them to agree before they can start using the app.
      #
      # If your license does not require such approval -- because it does not add any restrictions
      # beyond default copyright protections -- consider whether it would make sense to use `none`
      # instead; this will avoid prompting the user.

      publicDomain @8 :Util.LocalizedText;
      # Indicates that the app is placed in the public domain; you place absolutely no restrictions
      # on its use or distribution. The text is your public domain dedication statement. Please
      # note that public domain is not recognized in all jurisdictions, therefore using public
      # domain is widely considered risky. The Open Source Initiative recommends using a permissive
      # license like MIT's rather than public domain. unlicense.org provides resources to help you
      # use public domain; it is highly recommended that you read it before using this.
    }

    notices @9 :Util.LocalizedText;
    # Contains any third-party copyright notices that the app is required to display, for example
    # due to use of third-party open source libraries.
  }

  categories @10 :List(Category);
  # List of categories/genres to which this app belongs, sorted with best fit first. See the
  # `Category` enum below.
  #
  # You can list multiple categories, but note that as with all things the app market moderators
  # may ask you to make changes, e.g. if you list categories that don't fit or seem spammy.

  author :group {
    # Fields relating to the author of this app.
    #
    # The "author" might be a human, but could also be a company, or a pseudo-identity created to
    # represent the app itself.
    #
    # It is extremely important to users that they be able to verify the author's identity in a way
    # that is not susceptible to spoofing or forgery. Therefore, we *only* identify the author by
    # PGP key. Various PGP infrastructure exists which can be used to determine the author's
    # identity based on their PGP key. For example, Keybase.io has done a really good job of
    # connecting PGP keys to other Internet identities in a verifiable way.

    upstreamAuthor @19 :Text;
    # Name of the original primary author of this app, if it is different from the person who
    # produced the Sandstorm package. Setting this implies that the author connected to the PGP
    # signature only "ported" the app to Sandstorm.

    contactEmail @11 :Text;
    # Email address to contact for any issues with this app. This includes end-user support
    # requests as well as app store administrator requests, so it is very important that this be a
    # valid address with someone paying attention to it.

    pgpSignature @12 :Data;
    # PGP signature attesting responsibility for the app ID. This is a binary-format detached
    # signature of the following ASCII message (not including the quotes, no newlines, and
    # replacing <app-id> with the standard base-32 text format of the app's ID):
    #
    # "I am the author of the Sandstorm.io app with the following ID: <app-id>"
    #
    # You can create a signature file using `gpg` like so:
    #
    #     echo -n "I am the author of the Sandstorm.io app with the following ID: <app-id>" |
    #         gpg --sign > pgp-signature
    #
    # To learn how to set up gpg, visit Keybase (https://keybase.io) -- they have excellent
    # documentation and tools. Moreover, if you create a Keybase account for your key and follow
    # Keybase's instructions to link it to other social accounts (like your Github account), then
    # the Sandstorm app install flow and app market will present this information to the user as
    # "verified".
  }

  pgpKeyring @13 :Data;
  # A keyring in GPG keyring format containing all public keys needed to verify PGP signatures in
  # this manifest (as of this writing, there is only one: `author.pgpSignature`).
  #
  # To generate a keyring containing just your public key, do:
  #
  #     gpg --export <key-id> > keyring
  #
  # Where `<key-id>` is a PGP key ID or email address associated with the key.

  description @14 :Util.LocalizedText;
  # The app's description in Github-flavored Markdown format, to be displayed e.g.
  # in an app store. Note that the Markdown is not permitted to cotnain HTML nor image tags (but
  # you can include a list of screenshots separately).

  shortDescription @15 :Util.LocalizedText;
  # A very short (one-to-three words) description of what the app does. For example,
  # "Document editor", or "Notetaking", or "Email client". This will be displayed under the app
  # title in the grid view in the app market.

  screenshots @16 :List(Screenshot);
  # Screenshots to use for marketing purposes.

  struct Screenshot {
    width @0 :UInt32;
    height @1 :UInt32;
    # Width and height of the screenshot in "device-independent pixels". The actual width and
    # height of the image is in the image data, but this width and height is used to decide how
    # much to scale the image for it to "look right". Typically, a screenshot taken on a high-DPI
    # display should specify this width and height as half of the actual image width and height.
    #
    # The market is under no obligation to display images with any particular size; these are just
    # hints.

    union {
      unknown @2 :Void;
      # Unknown file format.

      png @3 :Data;
      # PNG-encoded image data. Usually preferred for screenshots.

      jpeg @4 :Data;
      # JPEG-encoded image data. Preferred for screenshots that contain photographs or the like.
    }
  }

  changeLog @17 :Util.LocalizedText;
  # Documents the history of changes in Github-flavored markdown format (with the same restrictions
  # as govern `description`). We recommend formatting this with an H1 heading for each version
  # followed by a bullet list of changes.
}

struct OsiLicenseInfo {
  id @0 :Text;
  # The file name of the license at opensource.org, i.e. such that the URL can be constructed as:
  #     http://opensource.org/licenses/<id>

  title @1 :Text;
  # Display title for app market. E.g. "Apache License 2.0".

  requireSource @2 :Bool = false;
  # Whether or not you are required to provide a `codeUrl` when specifying this license.
}

annotation osiInfo @0x9476412d0315d869 (enumerant) :OsiLicenseInfo;
# Annotation applied to each item in the OpenSourceLicense enum.

enum OpenSourceLicense {
  # Identities an OSI-approved Open Source license. Apps which claim to be "open source" must use
  # one of these licenses.

  invalid @0;  # Sentinel value; do not choose this.

  # Recommended licenses, especially for new code. These four licenses cover the spectrum of open
  # source license mechanics and are widely recognized and understood.
  mit        @1 $osiInfo(id = "MIT"       , title = "MIT License");
  apache2    @2 $osiInfo(id = "Apache-2.0", title = "Apache License v2");
  gpl3       @3 $osiInfo(id = "GPL-3.0"   , title = "GNU GPL v3", requireSource = true);
  agpl3      @4 $osiInfo(id = "AGPL-3.0"  , title = "GNU AGPL v3", requireSource = true);

  # Other popular general-purpose licenses.
  bsd3Clause @5 $osiInfo(id = "BSD-3-Clause", title = "BSD 3-Clause");
  bsd2Clause @6 $osiInfo(id = "BSD-2-Clause", title = "BSD 2-Clause");
  gpl2       @7 $osiInfo(id = "GPL-2.0"     , title = "GNU GPL v2", requireSource = true);
  lgpl2      @8 $osiInfo(id = "LGPL-2.1"    , title = "GNU LGPL v2.1", requireSource = true);
  lgpl3      @9 $osiInfo(id = "LGPL-3.0"    , title = "GNU LGPL v3", requireSource = true);
  isc       @10 $osiInfo(id = "ISC"         , title = "ISC License");

  # Popular licenses associated with specific languages.
  artistic2 @11 $osiInfo(id = "Artistic-2.0", title = "Artistic License v2");
  python2   @12 $osiInfo(id = "Python-2.0"  , title = "Python License v2");
  php3      @13 $osiInfo(id = "PHP-3.0"     , title = "PHP License v3");

  # Popular licenses associated with specific projects or companies.
  mpl2      @14 $osiInfo(id = "MPL-2.0" , title = "Mozilla Public License v2", requireSource = true);
  cddl      @15 $osiInfo(id = "CDDL-1.0", title = "CDDL", requireSource = true);
  epl       @16 $osiInfo(id = "EPL-1.0" , title = "Eclipse Public License", requireSource = true);
  cpal      @17 $osiInfo(id = "CPAL-1.0" , title = "Common Public Attribution License", requireSource = true);
  zlib      @18 $osiInfo(id = "Zlib" , title = "Zlib/libpng License");

  # Is your preferred license not on the list? We are happy to add any OSI-approved license; that
  # is, anything on this page:
  #     http://opensource.org/licenses/alphabetical
  #
  # Feel free to send a pull request adding yours.
}

struct AppId {
  id0 @0 :UInt64;
  id1 @1 :UInt64;
  id2 @2 :UInt64;
  id3 @3 :UInt64;
}

struct PackageId {
  id0 @0 :UInt64;
  id1 @1 :UInt64;
}

struct VerifiedInfo {
  # `spk verify --capnp` writes this to stdout. Also, `spk verify --details` writes a JSON version
  # of this, with large `Data` and `Text` fields removed and `LocalizedText` simplified to their
  # `defaultText`.

  appId @0 :AppId;
  packageId @1 :PackageId;
  # App and package ID computed from package file signature and hash.

  title @2 :Util.LocalizedText;
  version @3 :UInt32;
  marketingVersion @4 :Util.LocalizedText;

  authorPgpKeyFingerprint @5 :Text;

  metadata @6 :Metadata;
  # Stuff extracted directly from manifest.
}

struct CategoryInfo {
  title @0 :Text;
}

annotation categoryInfo @0x8d51dd236606d205 (enumerant) :CategoryInfo $Go.name("CategoryInfoAnnotation");

enum Category {
  # ------------------------------------
  # "Meta": communication & coordination

  productivity @1 $categoryInfo(title = "Productivity");
  # Apps which manage productivity -- i.e. the "meta" apps you use to "get organized", NOT the apps
  # you use to actually produce content.
  #
  # Examples: Note-taking, to-dos, calendars, kanban boards, project management, team management.
  #
  # NON-examples: Document editors (-> office), e-mail (-> communications).

  communications @2 $categoryInfo(title = "Communications");
  # Email, chat, conferencing, etc. Things that you use primarily to communicate, not to organize.

  social @3 $categoryInfo(title = "Social");
  # Social networking. Overlaps with communication, but focuses on organizing a network of people
  # and surfacing content and interactions from your network that aren't explicitly addressed
  # to you.

  # ------------------------------------
  # Content creation

  webPublishing @4 $categoryInfo(title = "Web Publishing");
  # Tools for publishing web sites and blogs.

  office @5 $categoryInfo(title = "Office");
  # Tools for the office: editors for documents, spreadsheets, presentations, etc.

  developerTools @6 $categoryInfo(title = "DevTools");
  # Tools for software engineering: source control, test automation, compilers, IDEs, etc.

  science @7 $categoryInfo(title = "Science");
  # Tools for scientific / academic pursuits: data gathering, data processing, paper publishing,
  # etc.

  graphics @10 $categoryInfo(title = "Graphics");
  # Tools for creating graphics / visual art.

  # ------------------------------------
  # Content consumption

  media @8 $categoryInfo(title = "Media");
  # Content *consumption*: Apps that aren't used to create content, but are used to display and
  # consume it. Music players, photo galleries, video, feed readers, etc.

  games @9 $categoryInfo(title = "Games");
  # Games.

  # ------------------------------------

  other @0 $categoryInfo(title = "Other");
  # Use if nothing else fits -- but consider sending us a pull request to add a better category!
}

# ==============================================================================
# Below this point is not interesting to app developers.
#
# TODO(cleanup): Maybe move elsewhere?

struct KeyFile {
  # A public/private key pair, as generated by libsodium's crypto_sign_keypair.
  #
  # The keyring maintained by the spk tool contains a sequence of these.
  #
  # TODO(someday):  Integrate with desktop environment's keychain for more secure storage.

  publicKey @0 :Data;
  privateKey @1 :Data;
}

const magicNumber :Data = "\x8f\xc6\xcd\xef\x45\x1a\xea\x96";
# A sandstorm package is a file composed of two messages: a `Signature` and an `Archive`.
# Additionally, the whole file is XZ-compressed on top of that, and the XZ data is prefixed with
# `magicNumber`.  (If a future version of the package format breaks compatibility, the magic number
# will change.)

struct Signature {
  # Contains a cryptographic signature of the `Archive` part of the package, along with the public
  # key used to verify that signature.  The public key itself is the application ID, thus all
  # packages signed with the same key will be considered to be different versions of the same app.

  publicKey @0 :Data;
  # A libsodium crypto_sign public key.
  #
  # libsodium signing public keys are 32 bytes.  The application's ID is simply a textual
  # representation of this key.

  signature @1 :Data;
  # libsodium crypto_sign signature of the crypto_hash of the `Archive` part of the package
  # (i.e. the package file minus the header).
}

struct Archive {
  # A tree of files.  Used to represent the package contents.

  files @0 :List(File);

  struct File {
    name @0 :Text;
    # Name of the file.
    #
    # Must not contain forward slashes nor NUL characters.  Must not be "." nor "..".  Must not
    # be the same as any other file in the directory.

    lastModificationTimeNs @5 :Int64;
    # Modification timestamp to apply to the file after unpack. Measured in nanoseconds.

    union {
      regular @1 :Data;
      # Content of a regular file.

      executable @2 :Data;
      # Content of an executable.

      symlink @3 :Text;
      # Symbolic link path.  The link will be interpreted in the context of the sandbox, where the
      # archive itself mounted as the root directory.

      directory @4 :List(File);
      # A subdirectory containing a list of files.
    }
  }
}
using Go = import "/go.capnp";
$Go.package("spk");
$Go.import("zenhack.net/go/sandstorm/capnp/spk");
